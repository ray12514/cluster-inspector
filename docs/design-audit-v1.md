# Cluster Inspector design audit before v1

This audit applies the codebase-design lens to `cluster-inspector` as a whole:
small interfaces, deep modules, explicit seams, policy in data, and platform
variation behind adapters.

No v1 release has shipped. Findings below should be fixed directly when they
are correct; do not add compatibility paths for pre-v1 shapes.

## Product boundary

`cluster-inspector` produces observed system facts in `profile.yaml`. It does
not choose the stack, render Spack workspaces, call Spack, choose install/cache
roots, or decide whether a candidate external should be used.

The facts-versus-policy split is:

| Information | Owner | Notes |
|---|---|---|
| Available compilers, MPI providers, GPU toolkits, fabric, filesystem candidates | `cluster-inspector` profile facts | Observed or inferred with evidence. |
| Install tree, cache roots, view roots, module roots, Spack root | `deployment.yaml` consumed by `stack-composer` | Installer-chosen; never auto-derived from profile. |
| Externalization posture, package defaults, provider preference | `defaults.yaml` / `stack.yaml` | Render policy, not probe policy. |
| Rendered `packages.yaml`, `spack.yaml`, module layout | `stack-composer` templates | Generated from explicit inputs. |
| Concretize/install/buildcache | Downstream build tooling | `cluster-inspector` must not cross this seam. |

## Current module/seam assessment

| Area | Current interface | Assessment | Pre-v1 action |
|---|---|---|---|
| CLI commands | Cobra commands call probe functions and emit fragments | Reasonable. Thin enough, but command package should not gain probe policy. | Keep. |
| Profile model/validation | Go structs + embedded schema | Good seam. Schema remains owned by `stack-planning`. | Keep synced; do not fork schema semantics locally. |
| Provider inventory | `ProbeProviderInventory(candidates, hints)` | Correct seam. It hides generic Linux plus platform adapters behind one interface. | Keep as the only caller-facing provider probe. |
| Generic Linux provider discovery | compiler/MPI/GPU probes using discovery policy | Good direction, but still needs clearer policy semantics. | Tighten policy naming/docs and tests. |
| Platform discovery | `cray.go` adapter | Acceptable as an adapter, but it still contains several Cray-specific env/module facts. That is okay only inside the adapter. | Keep isolated; do not let generic probes call Cray helpers. |
| Discovery policy | `internal/resources/discovery_policy.yaml` | Useful, but name and field semantics can be mistaken for stack-selection policy. | Clarify as discovery vocabulary/candidate mapping only. Ambiguous module/filesystem field names were corrected after this audit. |
| Module verification | `verifyModules([]string)` returns env/command observations | Useful internal seam. It should remain non-login and policy-driven. | Keep; avoid adding provider-specific shell scripts. |
| Filesystem probing | emits `install_tree_candidates` from explicit candidate hints and observed shared roots | Profile may offer candidates, but install tree is installer-owned in `deployment.yaml`. | Keep observation-based shared filesystem candidates; do not add selected deployment paths here. |
| ROCm components | `component_candidates` table in discovery policy emits public `spack_components` | Improved from the smoke-path subset. Core baseline is inferred; additional ROCm candidates are evidence-gated. | Keep synced with `docs/rocm-component-discovery-v1.md` and review when the supported Spack floor changes. |
| System externals | focused list plus package-manager probes | Correct boundary if facts stay candidate-only. | Expand only through focus policy/hints, not broad `spack external find` behavior. |

## Discovery policy semantics

`internal/resources/discovery_policy.yaml` must mean:

- known clues for recognizing observed software;
- candidate mappings from observed roots/modules to profile facts;
- common Linux defaults that are broadly useful as probes; and
- platform-owned vocabulary used by provider adapters.

It must not mean:

- “use this external in the rendered stack”;
- “this is the complete stack policy”;
- “these module names are the only valid spellings”; or
- “these paths are installer choices.”

Field naming cleanup to consider before v1:

| Current field | Problem | Better direction |
|---|---|---|
| `module_name_prefixes` | Describes module-name clues, not install prefixes. | Keep. |
| `roots` | Can sound like chosen roots. | Keep only for discovery roots; document as candidate probe roots. |
| `shared_probe_roots` / `scratch_probe_roots` | Clearer than generic root names; these are probe roots only. | Keep install-tree candidates derived from shared roots only; scratch roots are for build-stage candidates. |
| `component_candidates` | Internal ROCm candidate table that emits public `spack_components` facts. | Keep version/support assumptions documented in `docs/rocm-component-discovery-v1.md`. |

## ROCm component finding

The current ROCm policy emits:

- `hip`
- `hsa-rocr-dev`
- `comgr`
- `rocblas`
- `hipblas`
- `hipsparse`
- `rocprim`
- `llvm-amdgpu`

This baseline is not the whole ROCm package family. The richer candidate-map
design is recorded in `docs/rocm-component-discovery-v1.md`. Current behavior:

1. Discover the ROCm root from module verification, `ROCM_PATH`, `hipcc`, and
   common roots such as `/opt/rocm`.
2. Emit the baseline components from a coherent ROCm root.
3. Emit additional ROCm package candidates only when configured evidence paths
   exist under that root.
4. Keep the component table in embedded data, not in Go probe logic.
5. Let `stack-composer` decide which candidates become `packages.yaml`
   externals based on stack/default policy.

Open design question: whether future component tables should be keyed by Spack
floor/version, ROCm version, or both. Revisit this when the supported Spack
floor changes.

## Filesystem finding

The profile schema currently requires `filesystem.install_tree_candidates`, and
the extraction map says candidates may come from hints or scans of known roots.
That is still only a candidate list.

The actual install tree, build stage, source cache, misc cache, view root,
module root, and publish root are installer-owned `deployment.yaml` values.
Therefore:

- `cluster-inspector` may report observed shared filesystems and whether a path
  is writable, lock-safe, and has space;
- `cluster-inspector` must not imply that a path is the selected install tree;
- `stack-composer` must render concrete paths only from `deployment.yaml`; and
- validation may compare `deployment.yaml` choices against profile candidates,
  but the profile must not choose for the installer.

Current rule: `cluster-inspector` derives install-tree candidates only from an
explicit `CLUSTER_INSPECTOR_INSTALL_TREE_CANDIDATE` hint and observed shared
probe roots. Scratch probe roots are reserved for build-stage candidates.

## Whole-codebase refactor candidates

Prioritize these in order:

1. **Clarify discovery policy contract.** Rename ambiguous fields where worth
   the churn, update docs/tests, and state that policy is discovery vocabulary.
   The first cleanup pass renamed module/filesystem policy fields; keep this
   rule active for future additions.
2. **Fix ROCm external candidate modeling.** Initial pass completed: the policy
   now has a broader candidate table, with optional components gated by
   evidence. Continue tuning from real ROCm systems.
3. **Correct filesystem candidate ownership.** Completed for fixed install-tree
   defaults. Continue to keep only observed/shared filesystem facts here.
4. **Generalize platform policy lookup.** Replace helper names like
   `crayPEPolicy()` in generic files with generic platform-policy lookup where
   possible. Keep Cray-specific facts inside the Cray adapter.
5. **Deepen the probe runner seam.** `run`, `commandPath`, filesystem checks,
   and env reads are currently global helpers. They work, but a probe context
   would improve testability for full-system sweeps without changing the public
   CLI.
6. **Keep provider adapters real.** `ProbeProviderInventory` is useful only if
   generic Linux and platform adapters stay behind it. Do not let callers stitch
   provider facts together themselves.

## Decision rule for future changes

Before implementation, classify a proposed change as one of:

- required by the current design;
- consistent but not yet planned;
- scope creep;
- conflicting with design; or
- design/doc update required first.

If ownership is unclear, stop at the doc/audit layer. Do not patch the Go code
until the owner is clear.
