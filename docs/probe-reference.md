# Probe Reference

`cluster-inspector` produces observed system facts for stack rendering. It does
not call Spack, render stack files, deploy releases, or infer stack intent.

## Commands

### `probe-system`

Collects system-wide facts from the current host and writes a system fragment.

```bash
cluster-inspector probe-system \
  --system example-linux \
  --hints systems/example-linux/inspector-hints.yaml \
  --output probes/system.yaml
```

System-wide probes cover OS/glibc, module tool, module candidates, fabric,
platform provider inventory, compiler providers, MPI providers, GPU toolkit
modules, focused ordinary system externals, and shared filesystem candidates.

Provider inventory is collected through one probe-layer seam:
`ProbeProviderInventory`. The default adapter is policy-driven generic Linux
discovery. Platform adapters, such as `cray-pe`, may add facts only when the
generic Linux path cannot safely infer them. Output remains generic
`compiler_providers`, `mpi_providers`, `gpu_toolkit_modules`, and
`provider_family` fields.

### `probe-node`

Collects facts for one node type and writes a node fragment.

```bash
cluster-inspector probe-node \
  --node-type gpu_compute_mi250x \
  --role runtime \
  --runner srun:partition=gpu,constraint=mi250x \
  --output probes/gpu_compute_mi250x.yaml
```

Supported runners are `this`, `srun`, and `pbsdsh`. Remote runners invoke the
same built binary on the target node class.

### `merge`

Merges one system fragment plus one or more node fragments into a deterministic
`profile.yaml`.

```bash
cluster-inspector merge \
  --system-fragment probes/system.yaml \
  --node probes/login.yaml \
  --node probes/cpu_compute.yaml \
  --output systems/example/profile.yaml
```

### `profile`

Runs the full pipeline: `probe-system`, each requested `probe-node`, `merge`,
schema validation, and semantic verification-ready profile emission.

```bash
cluster-inspector profile \
  --system example \
  --hints systems/example/inspector-hints.yaml \
  --node-type login=this:role=build_host \
  --node-type cpu_compute=srun:partition=cpu:role=runtime \
  --output systems/example/profile.yaml
```

### `verify`

Validates a generated or hand-written profile against the embedded schema and
semantic checks used by the v6 stack examples.

```bash
cluster-inspector verify systems/example/profile.yaml
```

## Hints

`inspector-hints.yaml` makes module discovery repeatable. Hints can include
known-good module names, exclude false positives, and add explicit extras when a
site module cannot be auto-discovered.

```yaml
schema_version: 1
compilers:
  include: [aocc/4.2]
  exclude_patterns: ["gcc-data/*"]
mpi:
  include: [openmpi/5.0.9]
gpu_toolkits:
  include: [rocm/6.0.0, cuda/12.5]
system_externals:
  include: [openssl, curl]
```

When `system_externals` is omitted, Cluster Inspector probes the default focused
set `openssl` and `curl`. Add an `include` list when a site wants a different
focused set. These are observed facts only; stack defaults decide whether the
renderer may use a discovered package as a Spack external.

Module verification uses non-login `bash -c`, initializes module machinery when
available, purges/resets module state, loads exactly the candidate modules, and
then probes environment variables and command paths.

## Discovery policy

Common discovery vocabulary lives in the embedded
`internal/resources/discovery_policy.yaml` resource. The word "policy" here
means discovery policy, not stack-selection policy. It defines:

- compiler, MPI, and GPU toolkit module-name segments;
- environment variables and commands used by clean-shell verification;
- common Linux toolkit roots such as ROCm and CUDA roots;
- focused system externals, currently `openssl` and `curl`;
- shared/scratch filesystem root candidates; and
- platform-owned prefixes and module tokens for provider adapters.

Generic probes should read this policy instead of hard-coding provider or site
vocabulary. Add a provider adapter only for platform facts that need special
interrogation beyond normal modules, commands, environment variables, and
filesystem evidence.

Discovery policy entries are clues and candidate mappings. They do not decide
whether `stack-composer` renders a candidate as a Spack external. That decision
belongs to explicit render inputs such as `defaults.yaml`, `stack.yaml`, and
`deployment.yaml`.

ROCm is more complicated than CUDA/NVHPC because Spack externalizes many ROCm
components as separate packages. The current component mapping is a candidate
table, not a complete claim about every ROCm package a site could externalize;
see `docs/design-audit-v1.md` before changing ROCm discovery.

On current Cray PE/CPE NVIDIA systems, hints should use the observed platform
modules, such as `PrgEnv-nvidia` plus the `nvidia/<version>` compiler module for
NVHPC and `cuda/<version>` for the CUDA toolkit. Compiler identity is emitted as
generic `compiler_providers[].name: nvhpc`; do not introduce a Cray-shaped
profile field for it.

## Caveats

Container fixtures validate parser/probe paths in known-shaped environments.
They do not replace test-cluster validation on real Cray or production HPC
systems.
