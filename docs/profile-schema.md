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
- `compiler_providers`
- `mpi_providers`
- `gpu_toolkit_modules`
- `system_externals`
- `filesystem`
- `node_types`

The inspector does not emit stack capabilities. Capability derivation belongs to
`stack-composer` because it depends on profile facts, template contracts, and
stack intent.

## Platform NVIDIA Naming

For current platform NVIDIA environments, record the compiler as an `nvhpc`
entry in `compiler_providers` because that is the Spack/compiler identity.
Module lists should use the actual modules observed on the system, such as
`PrgEnv-nvidia` plus `nvidia/<version>` on current Cray PE/CPE systems. CUDA
toolkit facts belong under `gpu_toolkit_modules.cudatoolkit` with the observed
CUDA toolkit module name, such as `cuda/<version>`.

## Local Fixtures

Tracked fixtures live under `tests/fixtures/`:

- `example-cray/`: Cray-style profile with login, CPU compute, MI250X, and MI300A node types.
- `example-linux/`: Generic Linux profile with site MPI plus AMD and NVIDIA GPU toolkit facts.

Ignored Docker fixtures under `docker/` can regenerate representative fragments,
but the tracked fixtures are the stable corpus used by `go test ./...`.
