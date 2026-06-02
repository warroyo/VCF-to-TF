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
	flagKubeconfig     string
	flagContext        string
	flagNoComments     bool
	flagRequiredOnly   bool
	flagMarkOptional   bool
	flagWait           bool
	flagClusterClass   string
	flagClusterClassNS string
)

// buildOpts assembles the render options from the global flags. comments comes
// from the caller (the interactive toggle or !--no-comments).
func buildOpts(comments bool) tf.Options {
	return tf.Options{
		Comments:     comments,
		MarkOptional: flagMarkOptional,
		RequiredOnly: flagRequiredOnly,
		Wait:         flagWait,
	}
}

// clusterGroup is the Cluster API group; a Cluster's topology variables are
// expanded from its ClusterClass.
const clusterGroup = "cluster.x-k8s.io"

// isClusterKind reports whether the resource is a Cluster API Cluster.
func isClusterKind(r k8s.APIResource) bool {
	return r.Group == clusterGroup && r.Kind == "Cluster"
}

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

Run with no arguments to interactively browse and pick an API type. Every type
is rendered as a kubernetes_manifest resource built straight from its API schema.`,
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
	root.PersistentFlags().BoolVar(&flagNoComments, "no-comments", false, "omit field documentation comments from the output")
	root.PersistentFlags().BoolVar(&flagRequiredOnly, "required-only", false, "emit only fields the API marks as required")
	root.PersistentFlags().BoolVar(&flagMarkOptional, "mark-optional", false, "keep all fields but replace descriptions with a terse '# optional' tag")
	root.PersistentFlags().BoolVar(&flagWait, "wait", false, "emit an example wait{} block from the type's status fields and conditions")
	root.PersistentFlags().StringVar(&flagClusterClass, "cluster-class", "", "ClusterClass name used to expand a Cluster's topology variables (Cluster kind only)")
	root.PersistentFlags().StringVar(&flagClusterClassNS, "cluster-class-namespace", "vmware-system-vks-public", "namespace to look up ClusterClasses in")

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
func generate(client *k8s.Client, r k8s.APIResource, opts tf.Options) (string, error) {
	doc, err := client.FetchOpenAPI(r)
	if err != nil {
		return "", fmt.Errorf("fetch schema for %s: %w", r.Kind, err)
	}
	hcl, err := tf.BuildExample(doc, r.Group, r.Version, r.Kind, opts)
	if err != nil {
		return "", fmt.Errorf("generate HCL for %s: %w", r.Kind, err)
	}
	return hcl, nil
}

// generateCluster expands a Cluster's topology variables from the named
// ClusterClass and renders the full Cluster manifest.
func generateCluster(ctx context.Context, client *k8s.Client, r k8s.APIResource, className string, opts tf.Options) (string, error) {
	cc, err := client.FindResource("clusterclasses")
	if err != nil {
		return "", fmt.Errorf("find ClusterClass API: %w", err)
	}
	raw, err := client.GetClusterClassVariables(ctx, cc, flagClusterClassNS, className)
	if err != nil {
		return "", fmt.Errorf("read ClusterClass %q in %s: %w", className, flagClusterClassNS, err)
	}
	vars, err := tf.ParseClusterVariables(raw)
	if err != nil {
		return "", err
	}
	return tf.BuildClusterExample(r.Group, r.Version, r.Kind, className, vars, opts)
}
