// Package k8s wraps client-go to discover the active context, enumerate the
// API resource types a cluster advertises (like `kubectl api-resources`) and
// fetch their OpenAPI v3 schema (the data behind `kubectl explain`).
package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// Client talks to the active cluster for discovery and schema introspection.
// It generates Terraform from the available API definitions, not from running
// instances. The one exception is ClusterClass, whose live status.variables
// hold the schema needed to expand a Cluster's topology variables.
type Client struct {
	discovery discovery.DiscoveryInterface
	dynamic   dynamic.Interface
}

// APIResource describes one selectable API type (one row of `api-resources`).
type APIResource struct {
	Group      string
	Version    string
	Resource   string // plural, e.g. "deployments"
	Kind       string // e.g. "Deployment"
	Namespaced bool
}

// GVR builds the GroupVersionResource for this entry.
func (r APIResource) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: r.Group, Version: r.Version, Resource: r.Resource}
}

// GroupVersion renders "group/version" (or just "version" for the core group).
func (r APIResource) GroupVersion() string {
	if r.Group == "" {
		return r.Version
	}
	return r.Group + "/" + r.Version
}

// openAPIPath is the OpenAPI v3 discovery path for this resource's group/version,
// e.g. "api/v1" (core) or "apis/apps/v1".
func (r APIResource) openAPIPath() string {
	if r.Group == "" {
		return "api/" + r.Version
	}
	return "apis/" + r.Group + "/" + r.Version
}

// NewClient builds a Client from the active kubeconfig context. kubeconfig and
// contextName are optional overrides; empty values fall back to the standard
// loading rules (KUBECONFIG env var, then ~/.kube/config) and current-context.
func NewClient(kubeconfig, contextName string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}

	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	restCfg, err := cc.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load rest config: %w", err)
	}

	dc, err := discovery.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("discovery client: %w", err)
	}

	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}

	return &Client{discovery: dc, dynamic: dyn}, nil
}

// ListAPIResources returns every gettable API type the cluster advertises
// (server-preferred versions), the equivalent of `kubectl api-resources`.
// Subresources (e.g. pods/status) are skipped.
func (c *Client) ListAPIResources() ([]APIResource, error) {
	lists, err := c.discovery.ServerPreferredResources()
	// Partial discovery errors are common (e.g. a flaky aggregated API); keep
	// whatever was returned rather than failing outright.
	if err != nil && len(lists) == 0 {
		return nil, fmt.Errorf("discover api resources: %w", err)
	}

	var out []APIResource
	for _, list := range lists {
		gv, parseErr := schema.ParseGroupVersion(list.GroupVersion)
		if parseErr != nil {
			continue
		}
		for _, r := range list.APIResources {
			if strings.Contains(r.Name, "/") {
				continue // subresource
			}
			if !hasVerb(r.Verbs, "get") {
				continue
			}
			out = append(out, APIResource{
				Group:      gv.Group,
				Version:    gv.Version,
				Resource:   r.Name,
				Kind:       r.Kind,
				Namespaced: r.Namespaced,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

// FetchOpenAPI returns the raw OpenAPI v3 JSON document for the resource's
// group/version. The document's components.schemas hold the field-level schema
// and descriptions (the data `kubectl explain` renders). The translation layer
// parses it and locates the schema matching the resource's GVK.
func (c *Client) FetchOpenAPI(r APIResource) ([]byte, error) {
	root := c.discovery.OpenAPIV3()
	paths, err := root.Paths()
	if err != nil {
		return nil, fmt.Errorf("open openapi v3: %w", err)
	}

	key := r.openAPIPath()
	gv, ok := paths[key]
	if !ok {
		return nil, fmt.Errorf("no OpenAPI v3 schema published at %q", key)
	}

	doc, err := gv.Schema("application/json")
	if err != nil {
		return nil, fmt.Errorf("fetch schema for %q: %w", key, err)
	}
	return doc, nil
}

// FindResource returns the discovered API type whose plural resource name
// matches (case-insensitively), e.g. "clusterclasses". Used to resolve a GVR
// for a live object read without hardcoding the API version.
func (c *Client) FindResource(plural string) (APIResource, error) {
	resources, err := c.ListAPIResources()
	if err != nil {
		return APIResource{}, err
	}
	for _, r := range resources {
		if strings.EqualFold(r.Resource, plural) {
			return r, nil
		}
	}
	return APIResource{}, fmt.Errorf("resource %q not found on this cluster", plural)
}

// ListNames lists the names of all objects of the given type in a namespace.
func (c *Client) ListNames(ctx context.Context, r APIResource, namespace string) ([]string, error) {
	list, err := c.dynamic.Resource(r.GVR()).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for i := range list.Items {
		names = append(names, list.Items[i].GetName())
	}
	sort.Strings(names)
	return names, nil
}

// GetClusterClassVariables fetches a ClusterClass and returns its
// status.variables as raw JSON for the translation layer to parse.
func (c *Client) GetClusterClassVariables(ctx context.Context, r APIResource, namespace, name string) ([]byte, error) {
	obj, err := c.dynamic.Resource(r.GVR()).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	vars, found, err := unstructured.NestedSlice(obj.Object, "status", "variables")
	if err != nil {
		return nil, fmt.Errorf("read status.variables: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("ClusterClass %q has no status.variables (not reconciled yet?)", name)
	}
	return json.Marshal(vars)
}

func hasVerb(verbs metav1.Verbs, want string) bool {
	for _, v := range verbs {
		if v == want {
			return true
		}
	}
	return false
}
