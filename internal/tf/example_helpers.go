package tf

import (
	"fmt"
	"strings"
)

// attr renders the left side of an HCL attribute assignment: `name = `.
func attr(name string) string { return name + " = " }

// isRequired reports whether key is listed in the schema node's required set.
func isRequired(node *schemaNode, key string) bool {
	for _, r := range node.Required {
		if r == key {
			return true
		}
	}
	return false
}

// toIfaceMap adapts a property map to the shape sortedKeys expects (it only
// reads keys), so output field order is deterministic.
func toIfaceMap(m map[string]*schemaNode) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k := range m {
		out[k] = nil
	}
	return out
}

// scalarPlaceholder returns an example value literal for a scalar schema node.
func scalarPlaceholder(node *schemaNode) string {
	if len(node.Enum) > 0 {
		if s, ok := node.Enum[0].(string); ok {
			return fmt.Sprintf("%q", s)
		}
		return fmt.Sprintf("%v", node.Enum[0])
	}
	switch node.Type {
	case "integer", "number":
		return "0"
	case "boolean":
		return "false"
	default:
		return `""`
	}
}

// typeLabel renders a short type annotation, e.g. "string", "integer (int64)",
// "array<object>", "map<string>".
func (n *schemaNode) typeLabel() string {
	switch n.Type {
	case "":
		return ""
	case "array":
		return "array"
	case "object":
		if n.isMap() {
			return "map"
		}
		return "object"
	default:
		if n.Format != "" {
			return fmt.Sprintf("%s (%s)", n.Type, n.Format)
		}
		return n.Type
	}
}

// enumList renders enum values as a comma-separated string for a comment.
func enumList(vals []interface{}) string {
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		parts = append(parts, fmt.Sprintf("%v", v))
	}
	return strings.Join(parts, ", ")
}

// wrapComment collapses internal whitespace and soft-wraps a description to
// ~90 columns so generated comments stay readable.
func wrapComment(text string) []string {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return nil
	}
	const width = 90

	var (
		lines []string
		cur   strings.Builder
	)
	for _, word := range strings.Fields(text) {
		if cur.Len() > 0 && cur.Len()+1+len(word) > width {
			lines = append(lines, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(word)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}
