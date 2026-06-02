package tf

import (
	"fmt"
	"strings"
)

// rolloutKinds are the workload kinds the kubernetes_manifest provider can wait
// on via `wait { rollout = true }` (equivalent to `kubectl rollout status`).
var rolloutKinds = map[string]bool{
	"Deployment":  true,
	"StatefulSet": true,
	"DaemonSet":   true,
}

// statusField is one scalar leaf under .status, addressable as a wait field path.
type statusField struct {
	path  string // e.g. "status.readyReplicas" or "status.loadBalancer.ingress[0].ip"
	desc  string
	node  *schemaNode
	value string // example regex placeholder
}

// waitBlock emits an example `wait {}` block (sibling of manifest) derived from
// the type's status schema. It surfaces the available rollout/condition/field
// waiters; the user keeps the checks they want and deletes the rest.
//
// Nothing is emitted when the type exposes no status fields, no conditions, and
// is not a rollout-capable workload.
func (g *generator) waitBlock(kind string, root *schemaNode) {
	status, _ := g.set.resolve(root.Properties["status"])

	rollout := rolloutKinds[kind]
	conditions := hasConditions(g.set, status)

	var fields []statusField
	if status != nil {
		g.collectStatusFields(status, "status", 1, &fields)
	}

	if !rollout && !conditions && len(fields) == 0 {
		return
	}

	g.comment("Optional wait{}: block create/update until status is ready.")
	g.comment("Keep the checks you want and delete the rest. Field values are regexes; \"*\" matches any value.")
	g.line("wait {")

	if rollout {
		g.comment("Wait for rollout to finish (like 'kubectl rollout status'). Usually sufficient on its own for workloads.")
		g.line("rollout = true")
	}

	if conditions {
		g.comment("Wait for a status condition. Repeat the block for multiple conditions.")
		g.line("condition {")
		g.line(`type   = ""`)
		g.line(`status = "True"`)
		g.line("}")
	}

	if len(fields) > 0 {
		g.line("fields = {")
		for _, f := range fields {
			g.waitFieldDoc(f)
			g.line(fmt.Sprintf("%q = %q", f.path, f.value))
		}
		g.line("}")
	}

	g.line("}")
}

// waitFieldDoc emits the description + type/enum annotation for a wait field,
// honoring the Comments option. MarkOptional's terse markers don't apply here.
func (g *generator) waitFieldDoc(f statusField) {
	if !g.opts.Comments {
		return
	}
	if f.desc != "" {
		g.comment(f.desc)
	}
	var ann []string
	if t := f.node.typeLabel(); t != "" {
		ann = append(ann, t)
	}
	if len(f.node.Enum) > 0 {
		ann = append(ann, "one of: "+enumList(f.node.Enum))
	}
	if len(ann) > 0 {
		g.line("# [" + strings.Join(ann, ", ") + "]")
	}
}

// collectStatusFields walks a status sub-schema and appends every scalar leaf as
// a wait-addressable field path. Arrays are indexed with [0]; open maps (dynamic
// keys) and the conditions[] array (handled by a condition block) are skipped.
func (g *generator) collectStatusFields(node *schemaNode, path string, depth int, out *[]statusField) {
	resolved, desc := g.set.resolve(node)
	if resolved == nil || depth > maxDepth {
		return
	}

	switch resolved.Type {
	case "string", "integer", "number", "boolean":
		*out = append(*out, statusField{path: path, desc: desc, node: resolved, value: waitValue(resolved)})

	case "object":
		if resolved.isMap() {
			return // dynamic keys can't be addressed by a fixed path
		}
		for _, key := range sortedKeys(toIfaceMap(resolved.Properties)) {
			if path == "status" && key == "conditions" {
				continue // surfaced via the condition{} block instead
			}
			g.collectStatusFields(resolved.Properties[key], path+"."+key, depth+1, out)
		}

	case "array":
		g.collectStatusFields(resolved.Items, path+"[0]", depth+1, out)
	}
}

// waitValue suggests an example value for a status field: the first enum value
// when the field is enumerated, otherwise "*" (match any value).
func waitValue(node *schemaNode) string {
	if len(node.Enum) > 0 {
		if s, ok := node.Enum[0].(string); ok {
			return s
		}
		return fmt.Sprintf("%v", node.Enum[0])
	}
	return "*"
}

// hasConditions reports whether status has a conditions[] array of objects with
// the standard type/status fields, i.e. it's waitable via a condition{} block.
func hasConditions(set *schemaSet, status *schemaNode) bool {
	if status == nil {
		return false
	}
	conds, _ := set.resolve(status.Properties["conditions"])
	if conds == nil || conds.Type != "array" {
		return false
	}
	items, _ := set.resolve(conds.Items)
	if items == nil || items.Type != "object" {
		return false
	}
	_, hasType := items.Properties["type"]
	_, hasStatus := items.Properties["status"]
	return hasType && hasStatus
}
