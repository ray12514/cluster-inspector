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
Cray PE inventory, compiler externals, MPI externals, GPU toolkit modules, and
shared filesystem candidates.

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
and schema validation.

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
semantic checks.

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
  include: [rocm/6.0.0]
```

Module verification uses non-login `bash -c`, initializes module machinery when
available, purges/resets module state, loads exactly the candidate modules, and
then probes environment variables and command paths.

## Caveats

Container fixtures validate parser/probe paths in known-shaped environments.
They do not replace test-cluster validation on real Cray or production HPC
systems.
