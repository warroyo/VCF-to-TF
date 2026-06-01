// Command vcf2tf inspects the available API types on the active Kubernetes
// context (like "kubectl api-resources") and generates a Terraform HCL skeleton
// from each type's OpenAPI schema (like "kubectl explain"), printed to stdout.
// It does not read live/deployed objects.
//
// Standard core/apps/batch resources render as native HashiCorp "kubernetes"
// provider resources (Strategy A). Everything else — including VCF custom
// resources — renders as the generic kubernetes_manifest resource (Strategy B).
package main

import (
	"fmt"
	"os"

	"vcf2tf/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
