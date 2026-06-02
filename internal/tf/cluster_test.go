package tf

import (
	"strings"
	"testing"
)

const sampleClusterClassVars = `[
  {
    "name": "vmClass",
    "definitions": [
      {
        "from": "default",
        "required": true,
        "schema": {
          "openAPIV3Schema": {
            "type": "string",
            "description": "VM class for nodes.",
            "x-metadata": {"annotations": {"x-vmware-vks/scope": "cluster,controlPlane,workers"}}
          }
        }
      }
    ]
  },
  {
    "name": "networks",
    "definitions": [
      {
        "from": "default",
        "required": false,
        "schema": {
          "openAPIV3Schema": {
            "type": "object",
            "description": "Cluster networking.",
            "properties": {
              "services": {
                "type": "object",
                "properties": {
                  "cidrBlocks": {"type": "array", "items": {"type": "string"}}
                }
              }
            }
          }
        }
      }
    ]
  },
  {
    "name": "kubeAPIServerFQDNs",
    "definitions": [
      {
        "from": "default",
        "required": false,
        "schema": {
          "openAPIV3Schema": {
            "type": "array",
            "description": "Deprecated: This variable is deprecated. Use kubernetes.endpointFQDNs instead.",
            "items": {"type": "string"}
          }
        }
      }
    ]
  }
]`

func TestParseClusterVariables(t *testing.T) {
	vars, err := ParseClusterVariables([]byte(sampleClusterClassVars))
	if err != nil {
		t.Fatal(err)
	}
	if len(vars) != 3 {
		t.Fatalf("expected 3 vars, got %d", len(vars))
	}

	byName := map[string]ClusterVariable{}
	for _, v := range vars {
		byName[v.Name] = v
	}

	if !byName["kubeAPIServerFQDNs"].Deprecated {
		t.Error("kubeAPIServerFQDNs should be flagged deprecated")
	}
	if byName["networks"].Deprecated {
		t.Error("networks should not be deprecated")
	}

	vm := byName["vmClass"]
	if !vm.Required {
		t.Error("vmClass should be required")
	}
	if strings.Join(vm.Scopes, ",") != "cluster,controlPlane,workers" {
		t.Errorf("vmClass scopes = %v", vm.Scopes)
	}

	// no annotation -> cluster-only
	if got := strings.Join(byName["networks"].Scopes, ","); got != "cluster" {
		t.Errorf("networks scopes = %q, want cluster", got)
	}
}

func TestBuildClusterExample(t *testing.T) {
	vars, err := ParseClusterVariables([]byte(sampleClusterClassVars))
	if err != nil {
		t.Fatal(err)
	}

	out, err := BuildClusterExample(nil, "cluster.x-k8s.io", "v1beta1", "Cluster", "my-cc", vars, Options{Comments: true, RequiredOnly: false})
	if err != nil {
		t.Fatal(err)
	}
	mustParse(t, out)

	for _, want := range []string{
		`resource "kubernetes_manifest" "example"`,
		`"cluster.x-k8s.io/v1beta1"`,
		`"Cluster"`,
		`"my-cc"`,
		"controlPlane = {",
		"machineDeployments = [",
		"overrides = [",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("cluster output missing %q\n---\n%s", want, out)
		}
	}

	// vmClass is scoped to all three sections -> appears 3 times (as a name).
	if n := strings.Count(out, `"vmClass"`); n != 3 {
		t.Errorf("vmClass should appear in 3 sections, got %d\n%s", n, out)
	}
	// networks has no scope -> cluster-global only -> once.
	if n := strings.Count(out, `"networks"`); n != 1 {
		t.Errorf("networks should appear once (cluster only), got %d\n%s", n, out)
	}
	// deprecated variable must be omitted entirely.
	if strings.Contains(out, "kubeAPIServerFQDNs") {
		t.Errorf("deprecated variable should be skipped\n%s", out)
	}
}

// clusterDoc is a minimal Cluster OpenAPI schema with a status carrying
// conditions[] and a couple scalar fields, used for the --wait block.
const clusterDoc = `{
  "components": {
    "schemas": {
      "io.x-k8s.cluster.v1beta1.Cluster": {
        "type": "object",
        "x-kubernetes-group-version-kind": [{"group": "cluster.x-k8s.io", "version": "v1beta1", "kind": "Cluster"}],
        "properties": {
          "status": {
            "type": "object",
            "properties": {
              "phase": {"type": "string"},
              "controlPlaneReady": {"type": "boolean"},
              "conditions": {
                "type": "array",
                "items": {"type": "object", "properties": {"type": {"type": "string"}, "status": {"type": "string"}}}
              }
            }
          }
        }
      }
    }
  }
}`

func TestBuildClusterExampleWait(t *testing.T) {
	vars, err := ParseClusterVariables([]byte(sampleClusterClassVars))
	if err != nil {
		t.Fatal(err)
	}

	out, err := BuildClusterExample([]byte(clusterDoc), "cluster.x-k8s.io", "v1beta1", "Cluster", "my-cc", vars, Options{Comments: true, Wait: true})
	if err != nil {
		t.Fatal(err)
	}
	mustParse(t, out)

	for _, want := range []string{
		"wait {",
		"condition {",                          // status.conditions[] -> condition block
		`"status.phase" = "*"`,
		`"status.controlPlaneReady" = "*"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("cluster wait output missing %q\n---\n%s", want, out)
		}
	}
}
