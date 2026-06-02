package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"

	"github.com/warroyo/VCF-to-TF/internal/k8s"
	"github.com/warroyo/VCF-to-TF/internal/tf"
)

// runInteractive shows the API-type picker and returns the generated HCL
// skeleton for the chosen type. Progress messages go to stderr so stdout stays
// reserved for the HCL output.
func runInteractive(ctx context.Context, client *k8s.Client) (string, error) {
	fmt.Fprintln(os.Stderr, "Discovering cluster APIs…")
	resources, err := client.ListAPIResources()
	if err != nil {
		return "", fmt.Errorf("list api resources: %w", err)
	}
	if len(resources) == 0 {
		return "", fmt.Errorf("no API resources discovered on this cluster")
	}

	items := make([]list.Item, 0, len(resources))
	for _, r := range resources {
		scope := "cluster"
		if r.Namespaced {
			scope = "namespaced"
		}
		items = append(items, pickItem{
			title: fmt.Sprintf("%-30s %s · %s", r.Kind, r.GroupVersion(), scope),
			desc:  "",
			value: r,
		})
	}

	startOpts := buildOpts(!flagNoComments)
	chosen, opts, err := runPicker("Select an API resource type", "type", items, startOpts)
	if err != nil {
		return "", err
	}
	res := chosen.(k8s.APIResource)

	// A Cluster's topology variables come from a ClusterClass; pick one.
	if isClusterKind(res) {
		className, o, err := pickClusterClass(ctx, client, opts)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(os.Stderr, "Generating Cluster from ClusterClass %s…\n", className)
		return generateCluster(ctx, client, res, className, o)
	}

	fmt.Fprintf(os.Stderr, "Generating Terraform example for %s…\n", res.Kind)
	return generate(client, res, opts)
}

// pickClusterClass lists the ClusterClasses in the configured namespace and
// prompts for one. Honors --cluster-class if already set. Returns the chosen
// name and the (possibly re-toggled) render options.
func pickClusterClass(ctx context.Context, client *k8s.Client, opts tf.Options) (string, tf.Options, error) {
	if flagClusterClass != "" {
		return flagClusterClass, opts, nil
	}

	cc, err := client.FindResource("clusterclasses")
	if err != nil {
		return "", opts, fmt.Errorf("find ClusterClass API: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Listing ClusterClasses in %s…\n", flagClusterClassNS)
	names, err := client.ListNames(ctx, cc, flagClusterClassNS)
	if err != nil {
		return "", opts, fmt.Errorf("list ClusterClasses in %s: %w", flagClusterClassNS, err)
	}
	if len(names) == 0 {
		return "", opts, fmt.Errorf("no ClusterClasses found in %s (set --cluster-class-namespace?)", flagClusterClassNS)
	}

	items := make([]list.Item, 0, len(names))
	for _, n := range names {
		items = append(items, pickItem{title: n, value: n})
	}
	picked, o, err := runPicker("Select a ClusterClass", "clusterclass", items, opts)
	if err != nil {
		return "", opts, err
	}
	return picked.(string), o, nil
}
