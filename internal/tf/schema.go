package tf

import (
	"encoding/json"
	"fmt"
	"strings"
)

// schemaNode is a (subset of an) OpenAPI v3 schema object as published by the
// Kubernetes API server. Only the fields needed to generate a Terraform
// skeleton are modelled.
type schemaNode struct {
	Type                 string                 `json:"type"`
	Description          string                 `json:"description"`
	Properties           map[string]*schemaNode `json:"properties"`
	Items                *schemaNode            `json:"items"`
	Required             []string               `json:"required"`
	Ref                  string                 `json:"$ref"`
	AllOf                []*schemaNode          `json:"allOf"`
	Enum                 []interface{}          `json:"enum"`
	Format               string                 `json:"format"`
	AdditionalProperties json.RawMessage        `json:"additionalProperties"`
	GVK                  []gvkExtension         `json:"x-kubernetes-group-version-kind"`
}

type gvkExtension struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

// openAPIDoc is the slice of the OpenAPI v3 document we care about.
type openAPIDoc struct {
	Components struct {
		Schemas map[string]*schemaNode `json:"schemas"`
	} `json:"components"`
}

// schemaSet holds all component schemas for a group/version and resolves $refs.
type schemaSet struct {
	schemas map[string]*schemaNode
}

func parseSchemaSet(doc []byte) (*schemaSet, error) {
	var d openAPIDoc
	if err := json.Unmarshal(doc, &d); err != nil {
		return nil, fmt.Errorf("parse openapi document: %w", err)
	}
	if len(d.Components.Schemas) == 0 {
		return nil, fmt.Errorf("openapi document has no component schemas")
	}
	return &schemaSet{schemas: d.Components.Schemas}, nil
}

// findKind returns the root schema whose x-kubernetes-group-version-kind matches
// the requested group/version/kind.
func (s *schemaSet) findKind(group, version, kind string) (*schemaNode, error) {
	for _, node := range s.schemas {
		for _, g := range node.GVK {
			if g.Group == group && g.Version == version && g.Kind == kind {
				return node, nil
			}
		}
	}
	return nil, fmt.Errorf("no schema found for %s/%s kind %s", group, version, kind)
}

// resolve follows a $ref (and a single-element allOf wrapper, the pattern the
// API server uses to attach a description to a referenced type) to the concrete
// schema node. The returned description prefers the wrapper's own description.
func (s *schemaSet) resolve(node *schemaNode) (*schemaNode, string) {
	if node == nil {
		return nil, ""
	}
	desc := node.Description

	// allOf: [ {$ref} ] with an optional sibling description.
	if node.Ref == "" && len(node.AllOf) == 1 && node.AllOf[0].Ref != "" {
		target, _ := s.resolve(node.AllOf[0])
		if desc == "" && target != nil {
			desc = target.Description
		}
		return target, desc
	}

	if node.Ref != "" {
		name := strings.TrimPrefix(node.Ref, "#/components/schemas/")
		if target, ok := s.schemas[name]; ok {
			resolved, tdesc := s.resolve(target)
			if desc == "" {
				desc = tdesc
			}
			return resolved, desc
		}
	}
	return node, desc
}

// isMap reports whether an object schema is an open map (additionalProperties)
// rather than a fixed set of properties — e.g. labels, annotations, data.
func (n *schemaNode) isMap() bool {
	if n.Type != "object" || len(n.Properties) > 0 {
		return false
	}
	if len(n.AdditionalProperties) == 0 {
		return false
	}
	return string(n.AdditionalProperties) != "false"
}

// isScalar reports whether the node renders as a single HCL attribute value.
func (n *schemaNode) isScalar() bool {
	switch n.Type {
	case "string", "integer", "number", "boolean":
		return true
	}
	return n.isMap()
}
