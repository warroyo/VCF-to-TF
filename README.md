# vcf2tf

Inspect the **available API types** on the active Kubernetes/VCF context (like
`kubectl api-resources`) and generate a Terraform HCL skeleton from each type's
**OpenAPI schema** (like `kubectl explain`), with field descriptions as inline
comments. It does **not** read live/deployed objects — it works from the API
definitions the cluster publishes.

- **Strategy A — native provider:** standard core/apps/batch/networking/rbac
  kinds render as native `kubernetes_*` provider resources (block style,
  snake_case fields).
- **Strategy B — manifest:** any other kind (VCF CRDs, etc.) renders as a
  generic `kubernetes_manifest` resource mirroring the exact API schema
  (object style, original field names).

Auth/kubeconfig setup is assumed to be handled upstream by VCF tooling; this
tool just reads the current context.

## Build

```sh
go mod tidy
go build -o vcf2tf .
```

## Usage

### Interactive (no args)

```sh
vcf2tf
```

Launches a keyboard-driven picker: browse/filter every API type the cluster
advertises (each tagged `native` or `manifest`), pick one, and the generated HCL
skeleton prints to stdout. Keys: `↑/↓` move, `/` filter, `enter` select,
`q`/`esc` quit. The TUI renders to stderr, so stdout stays clean and pipeable
(`vcf2tf > out.tf`).

### Direct (scripted)

```sh
vcf2tf example <resource_type>   # aliases: explain, get

# examples
vcf2tf example deployment
vcf2tf example secret
vcf2tf example tanzukubernetesclusters
vcf2tf example gateways.networking.tanzu.vmware.com
```

Flags: `--kubeconfig`, `--context`.

`<resource_type>` matches a kind, a plural resource name, or a fully qualified
`resource.group` form (case-insensitive, with prefix fallback) against the
cluster's discovered API types — so CRDs work without code changes.

Namespace is not a flag: this tool describes Kubernetes APIs, not namespaces.

## Layout

```
main.go                      entry point
internal/cli/root.go         cobra root + interactive entry + shared helpers
internal/cli/example.go      non-interactive `example <type>` + type matching
internal/cli/picker.go       bubbletea filterable list picker (renders to stderr)
internal/cli/interactive.go  pick API type -> generate skeleton
internal/k8s/client.go       context discovery, api-resources list, OpenAPI fetch
internal/tf/registry.go      GVK -> native provider resource type table
internal/tf/schema.go        OpenAPI v3 schema model + $ref resolution
internal/tf/example.go       schema -> HCL skeleton generator (BuildExample)
internal/tf/example_helpers.go  placeholders, type labels, comment wrapping
internal/tf/hcl.go           naming + singularization helpers
```

## Notes / limitations

- Output is canonically formatted via `hclwrite.Format` (equivalent to
  `terraform fmt`).
- Recursion is bounded (`maxDepth = 6`) so deeply nested or self-referential
  schemas produce a usable skeleton; truncated branches are marked with a
  comment.
- `metadata` is rendered as a slim `name`/`namespace`/`labels`/`annotations`
  stub rather than expanding the full `ObjectMeta` schema; `status` and
  `apiVersion`/`kind` (manifest carries the latter two literally) are not
  expanded.
- The native writer uses a heuristic: scalar fields and open maps (`labels`,
  `matchLabels`, ...) become HCL attributes, nested objects become blocks, and
  arrays of objects become repeated blocks (common plurals singularized,
  `containers` -> `container`). Uncommon provider block names may need a manual
  tweak; when in doubt the manifest strategy is exact.
- Each field is annotated with a `# [required, type, one of: ...]` line and the
  API's own description.
