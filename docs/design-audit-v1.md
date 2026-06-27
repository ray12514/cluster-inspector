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
| Discovery policy | `internal/resources/discovery_policy.yaml` | Useful, but name and field semantics can be mistaken for stack-selection policy. | Clarify as discovery vocabulary/candidate mapping only; consider field renames before v1. |
| Module verification | `verifyModules([]string)` returns env/command observations | Useful internal seam. It should remain non-login and policy-driven. | Keep; avoid adding provider-specific shell scripts. |
| Filesystem probing | emits `install_tree_candidates` from fixed candidate roots | Weakest seam. Profile may offer candidates, but install tree is installer-owned in `deployment.yaml`. Current `install_tree_paths` can look like a chosen path list. | Demote/remove guessed install-tree paths; keep observation-based shared filesystem candidates. |
| ROCm components | fixed `spack_components` list from policy | Incomplete. ROCm is split across many Spack packages; the current list is a smoke-path subset. | Replace with a fuller Spack-version-aware component candidate table and/or probe-backed component detection. |
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
| `module_prefixes` | Ambiguous with filesystem install prefixes. | Rename to `module_name_prefixes`. |
| `roots` | Can sound like chosen roots. | Keep only for discovery roots; document as candidate probe roots. |
| `install_tree_paths` | Sounds like installer choices and conflicts with `deployment.yaml` ownership. | Remove or rename to `shared_filesystem_probe_paths`; do not emit them as chosen install trees. |
| `spack_components` | Hides that ROCm component mapping may be version-sensitive and incomplete. | Move to a dedicated ROCm component table keyed by supported Spack/ROCm expectations. |

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

This is not enough to claim complete ROCm external coverage. It is only a
candidate subset for smoke tests. Before v1, ROCm should be handled as a richer
candidate map:

1. Discover the ROCm root from module verification, `ROCM_PATH`, `hipcc`, and
   common roots such as `/opt/rocm`.
2. Inspect the root for component directories/libraries where practical.
3. Emit only components that are supported by the active Spack version and
   either observed or intentionally inferred from a known ROCm layout.
4. Keep the component table in embedded data, not in Go probe logic.
5. Let `stack-composer` decide which candidates become `packages.yaml`
   externals based on stack/default policy.

Open design question: whether the component table should be keyed by Spack
floor/version, ROCm version, or both. The answer should be checked against the
supported Spack release before implementation.

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

Pre-v1 correction: remove or demote the fixed `install_tree_paths` policy list
so it cannot be mistaken for deployment input.

## Whole-codebase refactor candidates

Prioritize these in order:

1. **Clarify discovery policy contract.** Rename ambiguous fields where worth
   the churn, update docs/tests, and state that policy is discovery vocabulary.
2. **Fix ROCm external candidate modeling.** Add a fuller component table or
   probe-backed component detection. Do not hardcode a partial list as if it is
   complete.
3. **Correct filesystem candidate ownership.** Remove installer-looking
   defaults from discovery policy; keep only observed/shared filesystem facts.
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
