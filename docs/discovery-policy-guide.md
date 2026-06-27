# Discovery policy guide

`internal/resources/discovery_policy.yaml` is discovery vocabulary. It teaches
`cluster-inspector` how to recognize observed software on a target system. It
is not stack-selection policy and it does not decide which externals a rendered
Spack environment must use.

## When YAML is enough

Update only `discovery_policy.yaml` when an existing probe already has the
right behavior and only needs more vocabulary:

- another module segment or module-name prefix for an existing compiler, MPI
  provider, GPU toolkit, or fabric userspace library;
- another environment variable that points at an existing provider prefix;
- another version environment variable for an existing provider;
- another common root path for an existing provider;
- another ROCm component candidate with evidence paths; or
- another policy-backed system external using the existing external candidate
  fields.

Examples:

```yaml
mpi:
  - name: openmpi
    module_segments: [openmpi, ompi]
    env: [MPI_HOME, OPENMPI_ROOT]
```

```yaml
fabric:
  userspace_candidates:
    - name: libfabric
      commands: [fi_info]
      version_commands:
        - [fi_info, --version]
      module_segments: [libfabric, ofi]
```

## When code is required

Change Go code only when the behavior changes:

- a probe needs a new clean-shell verification flow;
- a new output field or profile shape is required;
- a new relationship between facts must be inferred;
- a provider needs platform interrogation that generic module/env/command
  probing cannot supply; or
- Stack Composer needs a new consumer contract and the schema/planning docs
  must change first.

If a change adds provider, platform, compiler, MPI, GPU toolkit, fabric, or
filesystem vocabulary directly to Go, stop and ask whether that vocabulary
belongs in this policy instead.

## Hints versus policy

Policy records known discovery vocabulary bundled into the binary. Hints are
operator-provided filters and explicit extras for a particular site or test
run.

Use hints when:

- a site wants to focus discovery on selected modules;
- a module exists but does not match the bundled policy vocabulary yet;
- a known bad module should be excluded; or
- an external must be supplied explicitly for a local test.

Use policy when the vocabulary is broadly useful enough to compile into the
binary.

## Fabric userspace

`fabric.userspace_candidates` drives discovery of MPI-adjacent fabric
libraries such as `libfabric` and `ucx`.

Cluster Inspector emits these as observed fabric facts:

```yaml
fabric:
  userspace:
    - name: libfabric
      version: "1.22.0"
      prefix: /opt/ofi/libfabric/1.22.0
```

Stack Composer decides later whether these facts become Spack package
externals. Cluster Inspector should not decide that a stack must use them.

For module-backed fabric userspace, add or update `module_segments` in policy
and use `fabric_userspace` hints to include or exclude site-specific modules:

```yaml
schema_version: 1
fabric_userspace:
  include:
    - libfabric/1.22.0
    - ucx/1.16.0
```
