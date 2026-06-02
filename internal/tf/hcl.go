// Package tf is the translation engine: given a Kubernetes API's OpenAPI v3
// schema, it generates a Terraform HCL skeleton (field placeholders plus the
// API's own field descriptions as comments). Every kind renders as a
// kubernetes_manifest with the exact API schema.
package tf

import "sort"

// sortedKeys returns map keys in deterministic order so output is stable.
func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
