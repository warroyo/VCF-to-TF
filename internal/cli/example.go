package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/warroyo/VCF-to-TF/internal/k8s"
)

func newExampleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "example <resource_type>",
		Aliases: []string{"explain", "get"},
		Short:   "Generate a Terraform HCL example for an API type (non-interactive)",
		Example: `  vcf2tf example deployment
  vcf2tf example secret
  vcf2tf example tanzukubernetesclusters
  vcf2tf example gateways.networking.tanzu.vmware.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			client, err := newClient()
			if err != nil {
				return err
			}

			resources, err := client.ListAPIResources()
			if err != nil {
				return fmt.Errorf("list api resources: %w", err)
			}

			res, err := matchResource(resources, query)
			if err != nil {
				return err
			}

			hcl, err := generate(client, res)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), hcl)
			return nil
		},
	}
	return cmd
}

// matchResource resolves a user query against the discovered API types. It
// matches (case-insensitively) the kind, the plural resource name, or a
// fully-qualified "resource.group" form. Ambiguous matches return an error
// listing the candidates.
func matchResource(resources []k8s.APIResource, query string) (k8s.APIResource, error) {
	q := strings.ToLower(query)

	var matches []k8s.APIResource
	for _, r := range resources {
		switch {
		case strings.EqualFold(r.Kind, query),
			strings.EqualFold(r.Resource, query),
			strings.EqualFold(r.Resource+"."+r.Group, query):
			matches = append(matches, r)
		}
	}

	// Fallback: case-insensitive prefix on kind/resource (e.g. "tanzukube").
	if len(matches) == 0 {
		for _, r := range resources {
			if strings.HasPrefix(strings.ToLower(r.Kind), q) ||
				strings.HasPrefix(strings.ToLower(r.Resource), q) {
				matches = append(matches, r)
			}
		}
	}

	switch len(matches) {
	case 0:
		return k8s.APIResource{}, fmt.Errorf("no API type matches %q (try the interactive picker: run with no arguments)", query)
	case 1:
		return matches[0], nil
	default:
		var names []string
		for _, m := range matches {
			names = append(names, fmt.Sprintf("%s.%s", m.Resource, m.GroupVersion()))
		}
		return k8s.APIResource{}, fmt.Errorf("%q is ambiguous; matches: %s", query, strings.Join(names, ", "))
	}
}
