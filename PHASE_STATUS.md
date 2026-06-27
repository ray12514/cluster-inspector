# Phase status

Tracks progress against the implementation plan in
`stack-planning/docs/cluster_inspector_stack_profile_design_v1.md`
§ Implementation Plan.

A phase is "complete" only when the design doc's acceptance criteria for
that phase pass on the fixture corpus.

## Phase 0 — Scaffolding

- [x] Go module + directory shape
- [x] Cobra CLI dispatch wired into `cmd/cluster-inspector/main.go`
- [x] All five subcommand stubs return "not yet implemented" cleanly
- [x] Makefile, golangci-lint config, pre-commit hook, AGENTS.md/AGENTS.md
- [x] Apache-2.0 LICENSE
- [x] Initial commit pushed to `github.com/ray12514/cluster-inspector`

## Phase 1 — Contract and Skeleton

Acceptance per design doc:
- A hand-written fixture profile validates.
- The tool can print a minimal homogeneous local profile skeleton.
- The built binary runs without the source checkout.

- [x] `internal/model/profile.go` — Go structs mirroring profile-v1.json
- [x] `internal/model/fragments.go` — system + per-node fragment structs
- [x] `internal/model/validation.go` — validate against embedded schema
- [x] `internal/model/evidence.go` — evidence/confidence types
- [x] `internal/output/yaml.go` — deterministic YAML emitter
- [x] `internal/output/json.go` — diagnostics JSON emitter
- [x] `internal/output/human.go` — human-readable summary
- [x] `internal/resources/profile_schema.json` — synced from stack-planning
- [x] Schema validation passes on a hand-written example profile
- [x] `cluster-inspector profile --system local --node-type login=this:role=both` emits a minimal local skeleton

## Phase 2 — System-Wide Probes

Acceptance per design doc:
- Generic Linux login-node system fragment is produced.
- Platform login-node system fragments emit generic `compiler_providers` and
  `mpi_providers`; Cray PE/CPE evidence is represented as provider inventory,
  not as a Cray-shaped fragment block.
- No probe requires Spack.

- [x] `internal/probes/system.go` — OS, glibc, hostname identity
- [x] `internal/probes/modules.go` — module-system detection (Lmod vs Tcl), MODULEPATH walk
- [x] `internal/probes/fabric.go` — fabric type, drivers, userspace libs
- [x] `internal/probes/cray.go` — minimal Cray PE/CPE platform evidence,
      emitted as generic compiler/MPI providers
- [x] `internal/probes/compiler.go` — generic compiler providers (gcc, aocc, intel, etc.)
- [x] `internal/probes/mpi.go` — generic MPI providers (openmpi, mpich, etc.)
- [x] `internal/probes/gpu.go` — GPU vendor detection, driver, toolkit ceiling
- [x] `internal/probes/filesystem.go` — install-tree, source-cache, buildcache candidates
- [x] `internal/resources/discovery_policy.yaml` + `internal/probes/provider_adapters.go` —
      shared discovery vocabulary and provider-adapter seam; generic Linux is
      the default adapter, Cray PE/CPE is a platform adapter
- [x] Evidence capture for every probe
- [x] `unknown` confidence handling when commands are missing
- [x] Table-driven parser tests landed for system / modules / cray / fabric / mpi / gpu / compiler / helpers (run via `go test ./...`); evidence on real Linux + Cray hosts still depends on running the binary there (deferred to the test cluster)

**Note on the `profile` all-in-one command.** `cluster-inspector profile`
now runs the same real Phase 3 pipeline as the lower-level commands:
`probe-system`, one `probe-node` per requested node type, deterministic
`merge`, and schema validation.

## Phase 3 — Node-Type Probes and Merge

Acceptance per design doc:
- Login + CPU compute + one GPU class merge into one valid profile.
- Two GPU node types with different arch labels merge without duplication.
- Re-running merge on the same fragments produces byte-identical YAML.

- [x] `internal/probes/node.go` — CPU target, GPU facts per-node, build-stage candidates
- [x] `internal/commands/probe_node.go` — `this:` / `srun:` / `pbsdsh:` runners
- [x] `internal/commands/merge.go` — deterministic merge of system + node fragments
- [x] `internal/commands/profile.go` — all-in-one profile orchestration uses real fragments
- [x] Byte-identical re-merge test

## Phase 4 — Module Hints and Clean-Shell Verification

Acceptance per design doc:
- Hints can exclude false compiler matches (e.g., `gcc-data/*`).
- Cray PE compiler and Cray MPICH modules verified with modules + prefixes.
- ROCm toolkit modules produce component external facts, not only `rocm/<v>`.

- [x] `internal/hints/schema.go` — `inspector-hints.yaml` shape
- [x] `internal/hints/apply.go` — include/exclude/extras semantics
- [x] Module enumeration + classification (extends `internal/probes/modules.go`)
- [x] Clean-shell verification of each candidate (controlled non-login shell, see AGENTS.md § Shell discipline)
- [x] Diagnostics for ambiguous/rejected/failed candidates

## Phase 5 — Full Stack Fixtures

Acceptance per design doc:
- `example-cray` fixture validates with CPU + MI250X + MI300A node types.
- `example-linux` fixture validates with site MPI and optional Spack-built MPI capability inputs.
- Render-time required profile facts are present for the v6 stack examples.

- [x] `tests/fixtures/example-cray/` — golden profile + node fragments
- [x] `tests/fixtures/example-linux/` — golden profile + node fragments
- [x] `internal/commands/verify.go` — full v6 schema + semantic checks (final form)
- [x] `docs/probe-reference.md` — per-probe usage + caveats
- [x] `docs/profile-schema.md` — link/mirror of profile-v1.json reference

## Maintenance notes

1. Read `AGENTS.md` first.
2. Treat the checked boxes above as the v1 completion record, not an active
   implementation queue.
3. For new work, update the design doc in `stack-planning` first when behavior
   changes the contract.
4. Run `make lint test validate` before committing.
5. Keep discovery vocabulary in `internal/resources/discovery_policy.yaml`
   unless a provider-specific adapter is required to interrogate platform facts.
6. Use `docs/design-audit-v1.md` before changing probe architecture, ROCm
   component discovery, filesystem candidates, or discovery-policy semantics.
