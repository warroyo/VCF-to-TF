# vcf2tf

[![ci](https://github.com/warroyo/VCF-to-TF/actions/workflows/ci.yml/badge.svg)](https://github.com/warroyo/VCF-to-TF/actions/workflows/ci.yml)
[![release](https://github.com/warroyo/VCF-to-TF/actions/workflows/release.yml/badge.svg)](https://github.com/warroyo/VCF-to-TF/actions/workflows/release.yml)
[![latest release](https://img.shields.io/github/v/release/warroyo/VCF-to-TF?sort=semver)](https://github.com/warroyo/VCF-to-TF/releases/latest)
[![go version](https://img.shields.io/github/go-mod/go-version/warroyo/VCF-to-TF)](go.mod)

`vcf2tf` reads the API types your cluster exposes and prints a Terraform block
for whichever one you pick. Every field comes with its description as a comment,
so you keep what you need and delete the rest.

It builds from the API schema, not your running objects. You get a blank
template to fill in, not a copy of something already deployed.

Standard Kubernetes types (Deployment, Secret, Service, and so on) come out as
native `kubernetes_*` resources. Everything else, including VCF custom
resources, comes out as a `kubernetes_manifest`.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/warroyo/VCF-to-TF/main/install.sh | sh
```

That grabs the right binary for your OS and architecture from the latest
release and drops it on your PATH. Prefer to do it yourself? Download a binary
from the [releases page](https://github.com/warroyo/VCF-to-TF/releases), or
build from source with `go install github.com/warroyo/VCF-to-TF/cmd/vcf2tf@latest`.

## Connect to VCFA

vcf2tf reads whatever context `kubectl` is currently pointed at, so before you
run it, log in and create a VCFA context with the VCF CLI.

You can target either scope:

- **Organization context** — sees every namespace and API available to your org.
  Use this to browse the full catalog of types.
- **Namespace context** — scoped to a single namespace. Use this when you want
  the APIs available within one namespace (for example, a specific project).

Create an **organization context** with the VCF CLI:

```sh
vcf context create vcfa \
  --endpoint "$VCFA_ENDPOINT" \
  --api-token "$VCF_CLI_VCFA_API_TOKEN" \
  --tenant-name "$VCFA_ORG" \
  --ca-certificate "$VCFA_CERT_PATH"
```

For a **namespace context**, add the namespace to the same command:

```sh
vcf context create vcfa \
  --endpoint "$VCFA_ENDPOINT" \
  --api-token "$VCF_CLI_VCFA_API_TOKEN" \
  --tenant-name "$VCFA_ORG" \
  --ca-certificate "$VCFA_CERT_PATH" \
  --namespace "$VCFA_NAMESPACE"
```

That sets your active `kubectl` context. Confirm with `kubectl config
current-context`, then run vcf2tf.

## Usage

### Browse the cluster

Run it with no arguments to search the full list of API types:

```sh
vcf2tf
```

Type to filter, arrow keys to move, `Enter` to pick, `q` to quit. Toggle output
options live while you browse:

- `c` — comments on/off
- `o` — optional-field tags (`# optional`) on/off
- `r` — required-only on/off

The current state shows in the title bar. The block prints to stdout.

### Generate one directly

Already know the type? Name it:

```sh
vcf2tf example deployment
vcf2tf example secret
vcf2tf example tanzukubernetescluster
vcf2tf example gateways.networking.tanzu.vmware.com
```

The name can be a kind (`Deployment`), a plural (`deployments`), or the fully
qualified form (`resource.group`). Send it straight to a file:

```sh
vcf2tf example deployment > deployment.tf
```

### VKS Clusters

A `Cluster` (`cluster.x-k8s.io`) is special: its `spec.topology` variables are
opaque in the Cluster's own schema. The real schemas live in the **ClusterClass**
it references, so vcf2tf reads that and expands the variables for you.

Each variable lands in every topology section its scope allows:

- `cluster` → `spec.topology.variables`
- `controlPlane` → `spec.topology.controlPlane.variables.overrides`
- `workers` → `spec.topology.workers.machineDeployments[].variables.overrides`

Interactively, picking `Cluster` prompts you for a ClusterClass. Scripted, pass
the name:

```sh
vcf2tf example cluster --cluster-class tkg-vsphere-default-v3.2.0
```

ClusterClasses are looked up in `vmware-system-vks-public` by default; override
with `--cluster-class-namespace`. The expansion is verbose (every variable, in
every section it supports) — `--no-comments` trims it sharply, and you delete
what you don't set.

## What you get

```sh
$ vcf2tf example deployment
```

```hcl
resource "kubernetes_deployment" "example" {
  # Standard object metadata.
  metadata {
    # Name of the object (required).
    name = ""
    # Namespace to create the object in.
    namespace = ""
  }
  spec {
    # Number of desired pods.
    # [integer (int32)]
    replicas = 0
    container {
      # [string]
      image = ""
      # [string, one of: Always, IfNotPresent, Never]
      image_pull_policy = "Always"
    }
  }
}
```

The comments come straight from the API docs, with a `# [type, required, allowed
values]` hint on each field. Output is already formatted, so `terraform fmt` has
nothing left to do.

Want just the HCL? Add `--no-comments`. Want only the fields the API requires?
Add `--required-only` (combine them for the leanest possible scaffold):

```sh
vcf2tf example deployment --no-comments --required-only
```

## Worth knowing

- The values are placeholders (`""`, `0`, `false`). Treat the output as a
  scaffold to edit, not something to apply as-is.
- Big nested types get cut off at a reasonable depth, with a comment marking
  where, so you don't end up with a thousand-line wall.
- `metadata` shows the fields you actually touch (name, namespace, labels,
  annotations) instead of the entire Kubernetes metadata schema.

## Flags

| Flag | Description |
| --- | --- |
| `--no-comments` | Skip the field documentation comments and print just the HCL. |
| `--required-only` | Emit only the fields the API marks as required. |
| `--mark-optional` | Keep every field, but tag optional ones with a terse `# optional` instead of full descriptions. |
| `--cluster-class` | ClusterClass name used to expand a `Cluster`'s topology variables. |
| `--cluster-class-namespace` | Where to find ClusterClasses (default `vmware-system-vks-public`). |
| `--kubeconfig` | Path to kubeconfig (defaults to `KUBECONFIG`, then `~/.kube/config`). |
| `--context` | Context to use (defaults to your current one). |
| `--version` | Print the version. |
