// Package k8s wraps client-go to discover the active context, enumerate the
// API resource types a cluster advertises (like `kubectl api-resources`) and
// fetch their OpenAPI v3 schema (the data behind `kubectl explain`).
package k8s

import (
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
)

// Client talks to the active cluster for discovery and schema introspection.
// It deliberately does not read live/deployed objects: this tool generates
// Terraform from the available API definitions, not from running instances.
type Client struct {
	discovery discovery.DiscoveryInterface
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

	return &Client{discovery: dc}, nil
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

func hasVerb(verbs metav1.Verbs, want string) bool {
	for _, v := range verbs {
		if v == want {
			return true
		}
	}
	return false
}
