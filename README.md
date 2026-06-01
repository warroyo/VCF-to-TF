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

It uses whatever context `kubectl` is already pointed at. Nothing else to set up.

## Usage

### Browse the cluster

Run it with no arguments to search the full list of API types:

```sh
vcf2tf
```

Type to filter, arrow keys to move, `c` to toggle comments on/off, `Enter` to
pick, `q` to quit. The block prints to stdout.

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

Want just the HCL? Add `--no-comments`:

```sh
vcf2tf example deployment --no-comments
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
| `--kubeconfig` | Path to kubeconfig (defaults to `KUBECONFIG`, then `~/.kube/config`). |
| `--context` | Context to use (defaults to your current one). |
| `--version` | Print the version. |
