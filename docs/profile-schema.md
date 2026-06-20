# Profile Schema

The durable `profile.yaml` contract is owned by `stack-planning`:

- Design: `stack-planning/docs/cluster_inspector_stack_profile_design_v1.md`
- Extraction map: `stack-planning/docs/cluster_inspector_profile_extraction_map_v1.md`
- Canonical schema: `stack-planning/schemas/profile-v1.json`

In the standard local checkout layout, `stack-planning` sits adjacent to this
repo at `../stack-planning/`.

This repository embeds a synced copy at:

- `internal/resources/profile_schema.json`

Refresh it with:

```bash
make sync-schema
```

## Required Top-Level Blocks

Profiles contain observed facts only:

- `schema_version`
- `system`
- `os`
- `fabric`
- `modules_system`
- `vendor_cray`
- `compilers_external`
- `mpi`
- `gpu_toolkit_modules`
- `filesystem`
- `node_types`

The inspector does not emit stack capabilities. Capability derivation belongs to
`stack-composer` because it depends on profile facts, template contracts, and
stack intent.

## Local Fixtures

Tracked fixtures live under `tests/fixtures/`:

- `example-cray/`: Cray-style profile with login, CPU compute, MI250X, and MI300A node types.
- `example-linux/`: Generic Linux profile with site MPI plus AMD and NVIDIA GPU toolkit facts.

Ignored Docker fixtures under `docker/` can regenerate representative fragments,
but the tracked fixtures are the stable corpus used by `go test ./...`.
