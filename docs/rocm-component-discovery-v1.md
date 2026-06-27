# ROCm component discovery before v1

This note records the ROCm component discovery decision for
`cluster-inspector`. It follows `docs/design-audit-v1.md`: the inspector emits
observed or intentionally inferred profile facts, while `stack-composer`
decides whether a candidate becomes a rendered Spack external.

## Sources checked

- Spack `v1.2.0` release: https://github.com/spack/spack/releases/tag/v1.2.0
- Spack packages `v2026.06.0` release referenced by Spack `v1.2.0`:
  https://github.com/spack/spack-packages/releases/tag/v2026.06.0
- Upstream package tree for `v2026.06.0`:
  https://github.com/spack/spack-packages/tree/v2026.06.0/repos/spack_repo/builtin/packages
- Representative package metadata:
  - `hip`: https://github.com/spack/spack-packages/blob/v2026.06.0/repos/spack_repo/builtin/packages/hip/package.py
  - `rocblas`: https://github.com/spack/spack-packages/blob/v2026.06.0/repos/spack_repo/builtin/packages/rocblas/package.py
  - `hipblas`: https://github.com/spack/spack-packages/blob/v2026.06.0/repos/spack_repo/builtin/packages/hipblas/package.py
  - `hipsparse`: https://github.com/spack/spack-packages/blob/v2026.06.0/repos/spack_repo/builtin/packages/hipsparse/package.py

Spack `v1.2.0` was published on 2026-06-21. Its release notes identify
`spack-packages` `v2026.06.0` as the relevant package release. That package
release contains a broader ROCm package family than the first smoke-path list,
including `hip*`, `hsa*`, `rocm*`, `roc*`, `rccl`, `miopen-hip`, `migraphx`,
`comgr`, and `half` packages.

## Decision

`internal/resources/discovery_policy.yaml` now stores ROCm
`component_candidates`. This is an internal discovery table, not the public
profile field. The public profile still emits:

```yaml
gpu_toolkit_modules:
  rocm:
    spack_components:
      - package: <spack-package-name>
        prefix: <absolute-prefix>
```

The component table has two classes:

| Class | Emission rule | Rationale |
|---|---|---|
| Required baseline | Always emitted from a coherent ROCm root. | Preserves the current profile contract and smoke fixtures for the core components Stack Composer already expects. |
| Evidence-gated optional | Emitted only when a configured file/directory exists under the ROCm prefix. | Avoids rendering `buildable: false` externals for ROCm packages that are not actually installed. |

This keeps the module deep: callers still use `rocmSpackComponents(prefix)`,
and the implementation owns the component-table details internally.

## Required baseline candidates

The current baseline remains:

- `hip`
- `hsa-rocr-dev`
- `comgr`
- `rocblas`
- `hipblas`
- `hipsparse`
- `rocprim`
- `llvm-amdgpu`

These match the existing profile examples and validation assumptions. They are
inferred from a detected ROCm root even in minimal smoke containers where the
real ROCm libraries are not present.

## Evidence-gated optional candidates

The policy also records broader ROCm package candidates from Spack packages
`v2026.06.0`, including:

- HIP-side packages: `hipcc`, `hipblas-common`, `hipblaslt`, `hipcub`,
  `hipfft`, `hipfort`, `hipify-clang`, `hiprand`, `hipsolver`, `hipsparselt`,
  `hip-tests`
- HSA/runtime packages: `hsa-amd-aqlprofile`, `hsakmt-roct`
- math/communication libraries: `rccl`, `rocalution`, `rocfft`, `rocrand`,
  `rocsolver`, `rocsparse`, `rocthrust`, `rocwmma`
- profiling/debug/tool packages: `rocprofiler-*`, `roctracer-*`,
  `rocm-dbgapi`, `rocm-debug-agent`, `rocm-gdb`, `rocm-smi-lib`
- compiler/build support: `rocm-cmake`, `rocm-clang-ocl`, `rocm-device-libs`,
  `rocm-openmp-extras`, `rocm-tensile`
- additional libraries/tools: `miopen-hip`, `migraphx`, `rocdecode`,
  `rocjpeg`, `rocmlir`, `rocpydecode`, `rocshmem`, `rocm-opencl`,
  `rocm-bandwidth-test`, `rocm-validation-suite`

Each optional entry has `probe_paths`. The inspector emits that component only
if one probe path exists under the detected ROCm prefix.

## Non-goals

- Do not call Spack to discover ROCm externals.
- Do not make Stack Composer use any emitted component; it is still only a
  candidate fact.
- Do not claim the table is exhaustive for every future Spack package release.
  It is aligned to Spack `v1.2.0` / package release `v2026.06.0` and should be
  reviewed when the supported Spack floor changes.

## Next hardening

Future work can improve this without changing the profile schema:

1. Add version guards if Spack package names or ROCm layout changes across
   supported versions.
2. Add real-system evidence from the first ROCm cluster test to tune
   `probe_paths`.
3. Consider a generated table from a pinned `spack-packages` release if manual
   maintenance becomes error-prone.
