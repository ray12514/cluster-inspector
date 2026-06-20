# cluster-inspector

The system-facts probe for the Spack stack generation system. Produces a
reviewed, commit-ready `profile.yaml` that the renderer can consume.

This repo implements the design specified in
[`stack-planning/docs/cluster_inspector_stack_profile_design_v1.md`](https://github.com/ray12514/stack-planning/blob/main/docs/cluster_inspector_stack_profile_design_v1.md)
and the per-field extraction map in
[`stack-planning/docs/cluster_inspector_profile_extraction_map_v1.md`](https://github.com/ray12514/stack-planning/blob/main/docs/cluster_inspector_profile_extraction_map_v1.md).

`stack-planning` is the source of truth for the design and for the
canonical `profile.yaml` schema. **If the design doc disagrees with this
implementation, the design is authoritative.** Either update the
implementation or open a doc fix first.

## Status

The v1 implementation phases are complete. The repo has the all-in-one profile
pipeline, lower-level probe/merge commands, module hints, clean-shell candidate
verification, semantic profile verification, full-stack fixtures, and
probe/schema reference docs.

## Layout

```
cluster-inspector/
  cmd/cluster-inspector/main.go      # entry point; wires cobra dispatch
  internal/
    commands/                         # one file per CLI subcommand
      profile.go probe_system.go probe_node.go merge.go verify.go
    model/                            # typed structs that mirror profile-v1.json
      profile.go fragments.go evidence.go validation.go
    probes/                           # per-fact probe implementations
      system.go modules.go cray.go compiler.go mpi.go
      gpu.go fabric.go filesystem.go node.go
    hints/                            # inspector-hints.yaml parsing + apply
      schema.go apply.go
    output/                           # emitters
      yaml.go json.go human.go
    resources/                        # embedded resource files (//go:embed)
      gpu_toolkit_ceilings.yaml
      rocm_components.yaml
      module_patterns.yaml
      profile_schema.json             # copy of profile-v1.json from stack-planning
  tests/fixtures/                     # golden inputs for tests
  docs/
    probe-reference.md
    profile-schema.md
```

## Build and run

```bash
# Build the binary
make build

# Show subcommands
./cluster-inspector --help

# Test
make test

# Full local validation
make lint test validate
```

Primary commands:

- `profile`: run `probe-system`, one `probe-node` per requested node type,
  merge, schema-validate, and write `profile.yaml`.
- `probe-system`: collect host-wide facts into a system fragment.
- `probe-node`: collect per-node-type CPU/GPU/build-stage facts into a node
  fragment using `this`, `srun`, or `pbsdsh`.
- `merge`: deterministically merge one system fragment plus node fragments.
- `verify`: validate a profile against the embedded schema and semantic checks.

## Companion / reference

- **Canonical design + schema:** [`ray12514/stack-planning`](https://github.com/ray12514/stack-planning)
- **Old Python reference implementation:** [`ray12514/clusterinspector`](https://github.com/ray12514/clusterinspector) — useful for mining probe-logic patterns (fabric detection, module enumeration, GPU detection). Different product shape, working probes.

## License

Apache-2.0. See [`LICENSE`](LICENSE).
