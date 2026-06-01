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
		strategy := "manifest"
		if tf.IsNative(r.Group, r.Version, r.Kind) {
			strategy = "native"
		}
		scope := "cluster"
		if r.Namespaced {
			scope = "namespaced"
		}
		items = append(items, pickItem{
			title: fmt.Sprintf("%-30s %s · %s · %s", r.Kind, r.GroupVersion(), scope, strategy),
			desc:  "",
			value: r,
		})
	}

	chosen, comments, err := runPicker("Select an API resource type", "type", items, !flagNoComments)
	if err != nil {
		return "", err
	}
	res := chosen.(k8s.APIResource)

	fmt.Fprintf(os.Stderr, "Generating Terraform example for %s…\n", res.Kind)
	return generate(client, res, comments)
}
