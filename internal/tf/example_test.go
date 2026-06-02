package tf

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
)

func mustParse(t *testing.T, src string) {
	t.Helper()
	p := hclparse.NewParser()
	_, diags := p.ParseHCL([]byte(src), "generated.tf")
	if diags.HasErrors() {
		t.Fatalf("generated HCL does not parse: %s\n---\n%s", diags.Error(), src)
	}
}

// minimal OpenAPI v3 document with one native-mappable kind (apps/v1
// Deployment) and one custom kind (example.com/v1 Widget).
const sampleDoc = `{
  "components": {
    "schemas": {
      "io.k8s.api.apps.v1.Deployment": {
        "type": "object",
        "x-kubernetes-group-version-kind": [{"group": "apps", "version": "v1", "kind": "Deployment"}],
        "properties": {
          "apiVersion": {"type": "string"},
          "kind": {"type": "string"},
          "metadata": {"type": "object"},
          "status": {"type": "object"},
          "spec": {"$ref": "#/components/schemas/io.k8s.api.apps.v1.DeploymentSpec"}
        }
      },
      "io.k8s.api.apps.v1.DeploymentSpec": {
        "type": "object",
        "required": ["selector", "template"],
        "properties": {
          "replicas": {"type": "integer", "format": "int32", "description": "Number of desired pods."},
          "paused": {"type": "boolean"},
          "selector": {
            "type": "object",
            "properties": {
              "matchLabels": {"type": "object", "additionalProperties": {"type": "string"}}
            }
          },
          "containers": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "name": {"type": "string", "description": "Container name."},
                "image": {"type": "string"},
                "imagePullPolicy": {"type": "string", "enum": ["Always", "IfNotPresent", "Never"]}
              }
            }
          }
        }
      },
      "com.example.v1.Widget": {
        "type": "object",
        "x-kubernetes-group-version-kind": [{"group": "example.com", "version": "v1", "kind": "Widget"}],
        "properties": {
          "apiVersion": {"type": "string"},
          "kind": {"type": "string"},
          "metadata": {"type": "object"},
          "spec": {
            "type": "object",
            "required": ["size"],
            "properties": {
              "size": {"type": "integer", "description": "How big the widget is."},
              "mode": {"type": "string", "enum": ["fast", "slow"]},
              "tags": {"type": "array", "items": {"type": "string"}}
            }
          }
        }
      }
    }
  }
}`

// Standard kinds (e.g. apps/v1 Deployment) render as kubernetes_manifest too,
// keeping the original API field names, descriptions, and enums.
func TestExampleCoreKindAsManifest(t *testing.T) {
	out, err := BuildExample([]byte(sampleDoc), "apps", "v1", "Deployment", Options{Comments: true, RequiredOnly: false})
	if err != nil {
		t.Fatal(err)
	}
	mustParse(t, out)

	for _, want := range []string{
		`resource "kubernetes_manifest" "example"`,
		"manifest = {",
		`kind       = "Deployment"`,
		"spec = {",
		"replicas = 0",
		"containers = [",               // array<object> -> object list, plural kept
		"imagePullPolicy",              // original field name, no snake_case
		"# Number of desired pods.",    // description comment
		"one of: Always, IfNotPresent", // enum annotation
		"matchLabels = {}",             // map -> attribute, original name kept
	} {
		if !strings.Contains(out, want) {
			t.Errorf("manifest output missing %q\n---\n%s", want, out)
		}
	}
}

func TestExampleManifest(t *testing.T) {
	out, err := BuildExample([]byte(sampleDoc), "example.com", "v1", "Widget", Options{Comments: true, RequiredOnly: false})
	if err != nil {
		t.Fatal(err)
	}
	mustParse(t, out)

	for _, want := range []string{
		`resource "kubernetes_manifest" "example"`,
		"manifest = {",
		`apiVersion = "example.com/v1"`,
		`kind       = "Widget"`,
		"spec = {",
		"size = 0",                 // original field name kept (no snake_case)
		"# How big the widget is.", // description comment
		`mode = "fast"`,            // enum -> first value as placeholder
		"tags = []",                // scalar array
	} {
		if !strings.Contains(out, want) {
			t.Errorf("manifest output missing %q\n---\n%s", want, out)
		}
	}
}

func TestExampleRequiredOnly(t *testing.T) {
	out, err := BuildExample([]byte(sampleDoc), "apps", "v1", "Deployment", Options{Comments: true, RequiredOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	mustParse(t, out)

	// spec.required = [selector, template]; everything else is optional.
	if !strings.Contains(out, "selector") {
		t.Errorf("required field selector missing\n%s", out)
	}
	for _, omit := range []string{"replicas", "paused", "containers", "imagePullPolicy"} {
		if strings.Contains(out, omit) {
			t.Errorf("optional field %q should be omitted in required-only mode\n%s", omit, out)
		}
	}
	// metadata keeps only name.
	if strings.Contains(out, "annotations") || strings.Contains(out, "labels") {
		t.Errorf("required-only metadata should be name-only\n%s", out)
	}
}

func TestExampleMarkOptional(t *testing.T) {
	out, err := BuildExample([]byte(sampleDoc), "apps", "v1", "Deployment", Options{Comments: true, MarkOptional: true})
	if err != nil {
		t.Fatal(err)
	}
	mustParse(t, out)

	// all fields still present
	for _, want := range []string{"replicas = 0", "containers = [", "selector = {"} {
		if !strings.Contains(out, want) {
			t.Errorf("mark-optional should keep all fields, missing %q\n%s", want, out)
		}
	}
	// terse marker present, full descriptions gone
	if !strings.Contains(out, "# optional") {
		t.Errorf("expected '# optional' markers\n%s", out)
	}
	if strings.Contains(out, "# Number of desired pods.") {
		t.Errorf("full descriptions should be replaced by markers\n%s", out)
	}
}

func TestExampleNoComments(t *testing.T) {
	out, err := BuildExample([]byte(sampleDoc), "apps", "v1", "Deployment", Options{Comments: false, RequiredOnly: false})
	if err != nil {
		t.Fatal(err)
	}
	mustParse(t, out)

	if strings.Contains(out, "#") {
		t.Errorf("expected no comments with comments=false, got:\n%s", out)
	}
	// structure must still be present
	for _, want := range []string{
		`resource "kubernetes_manifest" "example"`,
		"replicas = 0",
		"containers = [",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no-comments output missing %q\n---\n%s", want, out)
		}
	}
}
