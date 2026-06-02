package tf

import (
	"strings"
	"testing"
)

// doc with a custom kind carrying a rich status: an enum phase, a numeric field,
// a nested array (loadBalancer.ingress[].ip), a conditions[] array, and an open
// map (skipped because its keys are dynamic).
const statusDoc = `{
  "components": {
    "schemas": {
      "com.example.v1.App": {
        "type": "object",
        "x-kubernetes-group-version-kind": [{"group": "example.com", "version": "v1", "kind": "App"}],
        "properties": {
          "apiVersion": {"type": "string"},
          "kind": {"type": "string"},
          "metadata": {"type": "object"},
          "spec": {"type": "object", "properties": {"size": {"type": "integer"}}},
          "status": {
            "type": "object",
            "properties": {
              "phase": {"type": "string", "description": "Lifecycle phase.", "enum": ["Pending", "Running"]},
              "readyReplicas": {"type": "integer"},
              "loadBalancer": {
                "type": "object",
                "properties": {
                  "ingress": {"type": "array", "items": {"type": "object", "properties": {"ip": {"type": "string"}}}}
                }
              },
              "conditions": {
                "type": "array",
                "items": {"type": "object", "properties": {"type": {"type": "string"}, "status": {"type": "string"}}}
              },
              "annotations": {"type": "object", "additionalProperties": {"type": "string"}}
            }
          }
        }
      }
    }
  }
}`

func TestWaitFromStatus(t *testing.T) {
	out, err := BuildExample([]byte(statusDoc), "example.com", "v1", "App", Options{Comments: true, Wait: true})
	if err != nil {
		t.Fatal(err)
	}
	mustParse(t, out)

	for _, want := range []string{
		"wait {",
		"condition {",                                // conditions[] -> condition block
		`status = "True"`,
		"fields = {",
		`"status.phase" = "Pending"`,                 // enum -> first value
		`"status.readyReplicas" = "*"`,               // scalar -> any value
		`"status.loadBalancer.ingress[0].ip" = "*"`,  // nested array indexed
	} {
		if !strings.Contains(out, want) {
			t.Errorf("wait output missing %q\n---\n%s", want, out)
		}
	}
	// conditions[] handled by the condition block, not duplicated as a field path.
	if strings.Contains(out, "status.conditions") {
		t.Errorf("conditions[] should not appear as a wait field path\n%s", out)
	}
	// open maps can't be addressed by a fixed path.
	if strings.Contains(out, "status.annotations") {
		t.Errorf("open map should be skipped in wait fields\n%s", out)
	}
}

// When the CRD enumerates condition type/status values, the condition block
// seeds the first one and lists the rest as a comment instead of leaving blanks.
func TestWaitConditionEnum(t *testing.T) {
	const doc = `{
  "components": {
    "schemas": {
      "com.example.v1.App": {
        "type": "object",
        "x-kubernetes-group-version-kind": [{"group": "example.com", "version": "v1", "kind": "App"}],
        "properties": {
          "status": {
            "type": "object",
            "properties": {
              "conditions": {
                "type": "array",
                "items": {
                  "type": "object",
                  "properties": {
                    "type": {"type": "string", "enum": ["Ready", "Available", "Progressing"]},
                    "status": {"type": "string", "enum": ["True", "False", "Unknown"]}
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`
	out, err := BuildExample([]byte(doc), "example.com", "v1", "App", Options{Comments: true, Wait: true})
	if err != nil {
		t.Fatal(err)
	}
	mustParse(t, out)

	for _, want := range []string{
		`type = "Ready"`,                          // first enum value seeded
		"# one of: Ready, Available, Progressing", // remaining listed
		`status = "True"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("condition enum output missing %q\n---\n%s", want, out)
		}
	}
}

func TestWaitRolloutKind(t *testing.T) {
	// Deployment is rollout-capable; wait should offer rollout even with the
	// status-less sample doc.
	out, err := BuildExample([]byte(sampleDoc), "apps", "v1", "Deployment", Options{Comments: true, Wait: true})
	if err != nil {
		t.Fatal(err)
	}
	mustParse(t, out)
	if !strings.Contains(out, "rollout = true") {
		t.Errorf("expected rollout = true for Deployment\n%s", out)
	}
}

func TestWaitOmittedWhenNothingToWaitOn(t *testing.T) {
	// Widget has no status and isn't a rollout kind -> no wait block at all.
	out, err := BuildExample([]byte(sampleDoc), "example.com", "v1", "Widget", Options{Comments: true, Wait: true})
	if err != nil {
		t.Fatal(err)
	}
	mustParse(t, out)
	if strings.Contains(out, "wait {") {
		t.Errorf("did not expect a wait block for a status-less, non-rollout kind\n%s", out)
	}
}

func TestWaitOffByDefault(t *testing.T) {
	out, err := BuildExample([]byte(statusDoc), "example.com", "v1", "App", Options{Comments: true})
	if err != nil {
		t.Fatal(err)
	}
	mustParse(t, out)
	if strings.Contains(out, "wait {") {
		t.Errorf("wait block should not appear unless Wait is set\n%s", out)
	}
}
