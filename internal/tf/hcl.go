// Package tf is the translation engine: given a Kubernetes API's OpenAPI v3
// schema, it generates a Terraform HCL skeleton (field placeholders plus the
// API's own field descriptions as comments). Standard kinds map to the native
// "kubernetes" provider; everything else maps to kubernetes_manifest.
package tf

import (
	"sort"
	"strings"
	"unicode"
)

// sortedKeys returns map keys in deterministic order so output is stable.
func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// camelToSnake converts a camelCase/PascalCase Kubernetes field name to the
// snake_case used by the Terraform kubernetes provider.
func camelToSnake(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			prevLower := i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]))
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if i > 0 && (prevLower || nextLower) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// blockSingulars maps a plural snake_case field name to the singular repeated
// block name the kubernetes provider expects. Unknown plurals pass through.
var blockSingulars = map[string]string{
	"containers":           "container",
	"init_containers":      "init_container",
	"ephemeral_containers": "ephemeral_container",
	"ports":                "port",
	"volumes":              "volume",
	"volume_mounts":        "volume_mount",
	"volume_devices":       "volume_device",
	"env":                  "env",
	"env_from":             "env_from",
	"image_pull_secrets":   "image_pull_secrets",
	"rules":                "rule",
	"subjects":             "subject",
	"tolerations":          "toleration",
}

func singularize(name string) string {
	if s, ok := blockSingulars[name]; ok {
		return s
	}
	return name
}
