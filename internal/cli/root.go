package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/warroyo/VCF-to-TF/internal/k8s"
	"github.com/warroyo/VCF-to-TF/internal/tf"
)

// Version is the build version, injected at release time via -ldflags
// "-X github.com/warroyo/VCF-to-TF/internal/cli.Version=...". Defaults to "dev" for local builds.
var Version = "dev"

// shared flags bound on the root command and read by every subcommand.
var (
	flagKubeconfig string
	flagContext    string
)

// NewRootCommand builds the top-level cobra command tree. Running the binary
// with no arguments launches the interactive picker; `example` supports
// scripted, non-interactive use.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:     "vcf2tf",
		Version: Version,
		Short:   "Generate Terraform HCL examples from a cluster's available APIs",
		Long: `vcf2tf inspects the available API types on the active Kubernetes context
(like "kubectl api-resources") and generates a Terraform HCL skeleton from each
type's OpenAPI schema (like "kubectl explain"), with field descriptions as
comments. It does not read live/deployed objects.

Run with no arguments to interactively browse and pick an API type. Standard
kinds map to the native "kubernetes" provider; custom resources map to the
generic kubernetes_manifest resource.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		// Bare invocation -> interactive flow.
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newClient()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			hcl, err := runInteractive(ctx, client)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), hcl)
			return nil
		},
	}

	root.PersistentFlags().StringVar(&flagKubeconfig, "kubeconfig", "", "path to kubeconfig (defaults to KUBECONFIG or ~/.kube/config)")
	root.PersistentFlags().StringVar(&flagContext, "context", "", "kubeconfig context to use (defaults to current-context)")

	root.AddCommand(newExampleCommand())
	return root
}

// newClient builds a k8s client from the shared connection flags.
func newClient() (*k8s.Client, error) {
	client, err := k8s.NewClient(flagKubeconfig, flagContext)
	if err != nil {
		return nil, fmt.Errorf("connect to cluster: %w", err)
	}
	return client, nil
}

// generate fetches the OpenAPI schema for an API type and renders the HCL
// skeleton.
func generate(client *k8s.Client, r k8s.APIResource) (string, error) {
	doc, err := client.FetchOpenAPI(r)
	if err != nil {
		return "", fmt.Errorf("fetch schema for %s: %w", r.Kind, err)
	}
	hcl, err := tf.BuildExample(doc, r.Group, r.Version, r.Kind)
	if err != nil {
		return "", fmt.Errorf("generate HCL for %s: %w", r.Kind, err)
	}
	return hcl, nil
}
