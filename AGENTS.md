# Agent guidance for `cluster-inspector`

This repo is the **Go implementation** of the system-facts probe specified
in the `stack-planning` planning library. It produces `profile.yaml` files
that the renderer (`stack-composer`) consumes.

## First things to read

| If you... | Start with |
|---|---|
| Need the high-level role and product boundary | `README.md` here, then `stack-planning/docs/cluster_inspector_stack_profile_design_v1.md` |
| Need to implement a specific profile-field probe | `stack-planning/docs/cluster_inspector_profile_extraction_map_v1.md` |
| Need to know what a valid `profile.yaml` looks like | `stack-planning/schemas/profile-v1.json` + tracked examples under `tests/fixtures/*/profile.yaml` |
| Need to change probe architecture or discovery policy | `docs/design-audit-v1.md` |
| Need to add discovery-policy vocabulary or hints | `docs/discovery-policy-guide.md` |
| Need to change ROCm component discovery | `docs/rocm-component-discovery-v1.md` |
| Want to mine probe-logic patterns from a working tool | The old Python `clusterinspector` at `github.com/ray12514/clusterinspector`. Clone adjacent to this repo if you'll port from it. |

If `stack-planning` isn't checked out locally, clone it to
`../stack-planning/` before doing implementation work. Schemas and
extraction rules live there; do not duplicate them here.

## Dependency philosophy

This is a **Go binary**, not a stdlib-only Go binary. The deployment
property we care about is "single self-contained binary on the target
with no language runtime dependency." Go satisfies that regardless of
what we vendor at build time. So: **prefer well-maintained Go libraries
over hand-rolling**, but keep the dependency list tight and justified.

Committed runtime dependencies (extend only with a recorded reason):

| Dependency | Used for |
|---|---|
| `github.com/spf13/cobra` | CLI multi-command dispatch (profile, probe-system, probe-node, merge, verify) |
| `gopkg.in/yaml.v3` | YAML read/write; deterministic output via stable key order |
| `github.com/santhosh-tekuri/jsonschema/v6` | Validate generated and hand-written profiles against `profile-v1.json` |
| `github.com/stretchr/testify` | `require`/`assert` in tests; clarity over hand-rolled `t.Fatalf` |

Stdlib for everything else: `embed` (resource files), `os/exec`
(subprocess to system probes), `log/slog` (structured logging),
`testing`, `encoding/json`, `regexp`. Do not pull in a third-party
logger, JSON library, or testing framework.

When adding a new dep:

1. Confirm it is **actively maintained** — recent commits, responsive
   issues, no known unresolved CVEs.
2. Prefer the canonical/most-used library in the Go ecosystem for that
   concern over niche alternatives.
3. Record the addition in this list with a one-line "used for" note.
4. Run `go mod tidy` and commit `go.mod` + `go.sum` together.

## Shell discipline (load-bearing)

The design doc has a hard rule that probes must use **non-login
shells**. Any subprocess that invokes a shell:

- Uses `bash -c '<probe>'` — **never** `bash -lc`, `bash --login`,
  `sh -l`, or anything that sources login startup files.
- For module verification, starts from a controlled non-login shell,
  clears or purges module state as needed, loads exactly the candidate
  modules, then probes. Does not inherit personal shell startup state.

Read `stack-planning/docs/cluster_inspector_stack_profile_design_v1.md`
§ Shell Invocation Discipline before writing any subprocess code. The
rule exists because login shells silently swap default programming
environments on Cray and similar sites, which makes probe output wrong
in ways that are hard to detect.

## Self-contained runtime

Once built, the binary must run on a target host with **no source
checkout, no `stack-planning` clone, and no network access**. All
resource files (the embedded schema, discovery policy, GPU toolkit
ceilings, ROCm component table, module patterns) live under `internal/resources/` and
are embedded via `//go:embed` at build time.

The build pipeline copies `profile-v1.json` from a local
`stack-planning` clone into `internal/resources/profile_schema.json`
before the build runs. There is no other way the binary obtains the
schema.

## What NOT to do

- Do not call Spack. The inspector never runs `spack spec`, `spack
  external find`, `spack config`, `spack concretize`, or `spack
  install`. Spack may exist on the target, but the inspector does not
  depend on it.
- Do not write `packages.yaml`, `spack.yaml`, modulefiles, lockfiles,
  or release manifests. Those belong to `stack-composer` and Spack
  itself.
- Do not generate template trees. `stack-composer scaffold-templates`
  may consume our profile corpus, but the inspector does not write
  templates.
- Do not infer stack intent from system facts. Report `cray-mpich`
  exists; do not decide that the stack should prefer it.
- Do not preserve Cray-shaped or obsolete intermediate contracts when the
  active profile contract is generic. System fragments and profile output
  must use generic provider inventory (`compiler_providers`,
  `mpi_providers`, `provider_family`). Cray-specific code is allowed only
  where it is strictly required to interrogate Cray PE/CPE facts
  (for example `/opt/cray`, `PrgEnv-*`, `cray-mpich`, or PE environment
  variables). If a Cray-specific branch remains, the nearby code or test
  must make clear why a generic Linux/module probe cannot provide that fact.
  Remove obsolete compatibility fields instead of transforming them later.
- Do not add provider, site, filesystem, GPU toolkit, compiler, MPI, or
  module-name literals directly to generic probes. Put broadly useful
  discovery vocabulary in `internal/resources/discovery_policy.yaml`; put
  platform-specific interrogation in a provider adapter behind
  `ProbeProviderInventory`. The generic Linux adapter is the default path.
- Do not silently fall back to login-shell behavior. If a non-login
  shell can't probe a fact, emit `unknown` with evidence — do not get
  the answer "right" by sourcing user dotfiles.
- Do not duplicate the canonical schema or design content here. Point
  at `stack-planning`.

## Required pre-change assessment

Before making a meaningful implementation change, write down the assessment in
the work log, PR description, issue, or a tracked doc. Do this before editing
code when the change touches probe behavior, profile shape, discovery policy,
provider adapters, module verification, or cross-repo contracts.

The assessment must answer:

1. **Requested change.** What behavior or contract is being changed?
2. **Design source.** Which `stack-planning` doc/schema or local agent guidance
   authorizes it? If none, update the design first.
3. **Ownership.** Is this an observed profile fact, installer-owned deployment
   input, stack/default policy, template behavior, or build-tool behavior?
4. **Scope classification.** Required by current design, consistent but
   unplanned, scope creep, or conflicts with design.
5. **Seam.** Which module/interface should own the behavior? Prefer the narrow
   public seam (`ProbeProviderInventory`, hints, model validation, etc.) over
   scattered call-site logic.
6. **Risks.** Does this add hardcoded site/vendor assumptions, duplicate
   another repo's responsibility, create new user-facing syntax, or weaken
   generic Linux behavior?
7. **Decision.** Implement now, document first then implement, defer, or reject.

Use the codebase-design vocabulary: keep modules deep, keep interfaces small,
put platform variation behind adapters, and keep discovery vocabulary in data
instead of spreading literals through generic probes.

## Implementation phase map

The design doc names five phases. They are implemented; keep this map as the
maintenance index for where each area lives:

| Phase | Scope | Code locations |
|---|---|---|
| **Phase 1 — Contract and Skeleton** | CLI dispatch, schema models, deterministic YAML output, validation harness for hand-written fixtures | `cmd/cluster-inspector/main.go`, `internal/commands/*.go`, `internal/model/*.go`, `internal/output/*.go`, `internal/resources/profile_schema.json` |
| **Phase 2 — System-Wide Probes** | OS, glibc, module-tool, fabric, filesystem, compiler, MPI, Cray PE, GPU toolkit probes; evidence capture | `internal/probes/{system,modules,cray,compiler,mpi,gpu,fabric,filesystem}.go` |
| **Phase 3 — Node-Type Probes And Merge** | CPU target, GPU facts, build-stage candidates; deterministic merge | `internal/probes/node.go`, `internal/commands/{probe_node,merge}.go` |
| **Phase 4 — Module Hints And Clean-Shell Verification** | Module enumeration, hints schema, include/exclude/extras, clean-shell verification | `internal/hints/*.go`, plus updates to `internal/probes/modules.go` |
| **Phase 5 — Full Stack Fixtures** | Golden fixtures, `verify` subcommand checks, documentation examples | `tests/fixtures/`, `internal/commands/verify.go` (final form), `docs/` |

All phase-planned command bodies are implemented. If future work adds a new
phase or changes the design contract, update this map and `PHASE_STATUS.md` in
the same change.

## Discovery policy and provider adapters

Provider discovery has one public seam inside the probe layer:
`ProbeProviderInventory(candidates, hints)`. Callers should not invoke
compiler, MPI, GPU, or platform provider probes independently unless they are
testing those adapters directly.

The seam currently runs:

1. `linux-generic`: policy-driven module, environment, command, and common
   filesystem discovery for ordinary HPC Linux systems.
2. `cray-pe`: Cray PE/CPE adapter logic only for facts the generic path cannot
   safely infer, such as program-environment module evidence, Cray MPICH
   prefixes, and PE-specific environment variables.

`internal/resources/discovery_policy.yaml` is the first place to update when
adding recognized compilers, MPI implementations, GPU toolkit roots,
system-external focus packages, scratch/shared filesystem roots, fabric
evidence, or platform-owned prefixes. Keeping this vocabulary in policy keeps
the probe modules generic and makes future platform adapters smaller.

The discovery policy is not stack-selection policy. It records discovery clues
and candidate external mappings: module-name segments/prefixes, environment
keys, command names, common roots, platform-owned prefixes, and component
mapping tables. It must not decide whether a rendered stack uses a candidate;
that decision belongs to `stack-composer` inputs such as `defaults.yaml`,
`stack.yaml`, and `deployment.yaml`.

## Build and test

```bash
# Initial setup
go mod tidy                                 # fetches the committed dep set
make sync-schema                            # refreshes embedded profile-v1.json

# Build
go build -o cluster-inspector ./cmd/cluster-inspector

# Run
./cluster-inspector --help

# Test
go test ./...
```

For probes that shell out to system commands, prefer table-driven tests
where the subprocess output is captured as a fixture under
`tests/fixtures/` and the probe is unit-tested against the fixture.
Avoid hitting the real host in unit tests; reserve that for an
explicit `e2e_test.go` build tag.

## End-to-end smoke against a real Linux container

A smoke pipeline that drives `cluster-inspector profile` + render +
`spack install` inside a Docker container lives outside this repo so
the integration target is replaceable without touching the inspector.
The persistent runtime is at `~/Development/smoke-runtime/`; the
orchestrator script and Dockerfile are colocated on the same host
(see `~/Development/smoke-runtime/README.md` for the actions and the
orchestrator's location). Run the smoke pipeline after changes that
affect probe output (system fact gathering, schema validation,
fragment merge) so the rendered profile still validates and downstream
consumers still concretize and install cleanly. Treat the smoke
runtime as a verification surface, not as a fixture this repo owns.

## When in doubt

The design doc in `stack-planning` is authoritative. If you're about to
make a design decision and the doc doesn't cover it, **stop and write
the doc fix first**, then implement against the fixed doc. Implementing
ahead of the spec creates drift that's expensive to unwind.
