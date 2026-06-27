// Package model holds the Go types that mirror the canonical profile.yaml
// schema. The authoritative shape is in
// stack-planning/schemas/profile-v1.json. The struct definitions here are
// derived from it and must stay in sync (run `make sync-schema` to refresh
// the embedded JSON; bump these types when the schema changes).
package model

// Profile mirrors the top-level profile.yaml document.
type Profile struct {
	SchemaVersion     int                 `json:"schema_version" yaml:"schema_version"`
	System            System              `json:"system" yaml:"system"`
	OS                OS                  `json:"os" yaml:"os"`
	Fabric            Fabric              `json:"fabric" yaml:"fabric"`
	ModulesSystem     ModulesSystem       `json:"modules_system" yaml:"modules_system"`
	CompilerProviders []CompilerProvider  `json:"compiler_providers,omitempty" yaml:"compiler_providers,omitempty"`
	MPIProviders      []MPIProvider       `json:"mpi_providers,omitempty" yaml:"mpi_providers,omitempty"`
	GPUToolkitModules *GPUToolkitModules  `json:"gpu_toolkit_modules,omitempty" yaml:"gpu_toolkit_modules,omitempty"`
	SystemExternals   []SystemExternal    `json:"system_externals,omitempty" yaml:"system_externals,omitempty"`
	Filesystem        Filesystem          `json:"filesystem" yaml:"filesystem"`
	NodeTypes         map[string]NodeType `json:"node_types" yaml:"node_types"`
}

type System struct {
	Name        string `json:"name" yaml:"name"`
	Family      string `json:"family" yaml:"family"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type OS struct {
	Name  string `json:"name" yaml:"name"`
	Major int    `json:"major" yaml:"major"`
	Minor *int   `json:"minor,omitempty" yaml:"minor,omitempty"`
	Glibc string `json:"glibc" yaml:"glibc"`
}

type Fabric struct {
	Type       string                 `json:"type" yaml:"type"`
	Generation string                 `json:"generation,omitempty" yaml:"generation,omitempty"`
	Drivers    []NamedPrefixVersioned `json:"drivers" yaml:"drivers"`
	Userspace  []NamedPrefixVersioned `json:"userspace,omitempty" yaml:"userspace,omitempty"`
}

type NamedPrefixVersioned struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
	Prefix  string `json:"prefix" yaml:"prefix"`
}

type ModulesSystem struct {
	Tool    string `json:"tool" yaml:"tool"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

// CompilerProvider is a compiler external tagged with its provider family
// (platform, site, system). PlatformFamily names an optional platform-specific
// provider such as cray-pe without making it part of the generic family axis.
type CompilerProvider struct {
	Name           string            `json:"name" yaml:"name"`
	Version        string            `json:"version" yaml:"version"`
	Prefix         string            `json:"prefix" yaml:"prefix"`
	ProviderFamily string            `json:"provider_family" yaml:"provider_family"`
	PlatformFamily string            `json:"platform_family,omitempty" yaml:"platform_family,omitempty"`
	Languages      []string          `json:"languages" yaml:"languages"`
	Modules        []string          `json:"modules,omitempty" yaml:"modules,omitempty"`
	Compilers      *CompilerCommands `json:"compilers,omitempty" yaml:"compilers,omitempty"`
}

// CompilerCommands holds optional explicit driver paths.
type CompilerCommands struct {
	C       string `json:"c,omitempty" yaml:"c,omitempty"`
	Cxx     string `json:"cxx,omitempty" yaml:"cxx,omitempty"`
	Fortran string `json:"fortran,omitempty" yaml:"fortran,omitempty"`
}

// MPIProvider is an MPI external tagged with provider family. Either
// single-prefix (Prefix [+ Modules]) or per-compiler Flavors.
type MPIProvider struct {
	Name           string               `json:"name" yaml:"name"`
	Version        string               `json:"version" yaml:"version"`
	ProviderFamily string               `json:"provider_family" yaml:"provider_family"`
	PlatformFamily string               `json:"platform_family,omitempty" yaml:"platform_family,omitempty"`
	Prefix         string               `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	Modules        []string             `json:"modules,omitempty" yaml:"modules,omitempty"`
	Compiler       string               `json:"compiler,omitempty" yaml:"compiler,omitempty"`
	Compatibility  *MPICompatibility    `json:"compatibility,omitempty" yaml:"compatibility,omitempty"`
	Flavors        map[string]MPIFlavor `json:"flavors,omitempty" yaml:"flavors,omitempty"`
}

// MPICompatibility records which compilers an MPI provider supports.
type MPICompatibility struct {
	Compilers []string `json:"compilers,omitempty" yaml:"compilers,omitempty"`
}

// MPIFlavor is a per-compiler MPI build at a distinct prefix.
type MPIFlavor struct {
	Prefix  string   `json:"prefix" yaml:"prefix"`
	Modules []string `json:"modules" yaml:"modules"`
}

type GPUToolkitModules struct {
	ROCm        *ROCmToolkitModule  `json:"rocm,omitempty" yaml:"rocm,omitempty"`
	CUDAToolkit *CUDAToolkitModule  `json:"cudatoolkit,omitempty" yaml:"cudatoolkit,omitempty"`
	NVHPC       *NvhpcToolkitModule `json:"nvhpc,omitempty" yaml:"nvhpc,omitempty"`
}

type ROCmToolkitModule struct {
	Version         string           `json:"version" yaml:"version"`
	Module          string           `json:"module" yaml:"module"`
	Prefix          string           `json:"prefix" yaml:"prefix"`
	SpackComponents []SpackComponent `json:"spack_components" yaml:"spack_components"`
}

type CUDAToolkitModule struct {
	Version string `json:"version" yaml:"version"`
	Module  string `json:"module" yaml:"module"`
	Prefix  string `json:"prefix" yaml:"prefix"`
}

type NvhpcToolkitModule struct {
	Version string `json:"version" yaml:"version"`
	Module  string `json:"module" yaml:"module"`
	Prefix  string `json:"prefix" yaml:"prefix"`
}

type SpackComponent struct {
	Package string `json:"package" yaml:"package"`
	Prefix  string `json:"prefix" yaml:"prefix"`
}

// SystemExternal is an ordinary package external such as OpenSSL or curl.
type SystemExternal struct {
	Name           string             `json:"name" yaml:"name"`
	Version        string             `json:"version" yaml:"version"`
	Prefix         string             `json:"prefix" yaml:"prefix"`
	ProviderFamily string             `json:"provider_family" yaml:"provider_family"`
	Variants       string             `json:"variants,omitempty" yaml:"variants,omitempty"`
	Modules        []string           `json:"modules,omitempty" yaml:"modules,omitempty"`
	Detection      *ExternalDetection `json:"detection,omitempty" yaml:"detection,omitempty"`
}

// ExternalDetection records how an external fact was obtained.
type ExternalDetection struct {
	Confidence string `json:"confidence" yaml:"confidence"`
	Source     string `json:"source" yaml:"source"`
}

type Filesystem struct {
	InstallTreeCandidates []InstallTreeCandidate `json:"install_tree_candidates" yaml:"install_tree_candidates"`
	SourceCacheCandidate  string                 `json:"source_cache_candidate,omitempty" yaml:"source_cache_candidate,omitempty"`
	BuildcacheCandidate   string                 `json:"buildcache_candidate,omitempty" yaml:"buildcache_candidate,omitempty"`
}

type InstallTreeCandidate struct {
	Path         string `json:"path" yaml:"path"`
	Type         string `json:"type" yaml:"type"`
	LocksHonored bool   `json:"locks_honored" yaml:"locks_honored"`
	FreeGB       *int   `json:"free_gb,omitempty" yaml:"free_gb,omitempty"`
}

type NodeType struct {
	Role        string       `json:"role" yaml:"role"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
	CPU         CPU          `json:"cpu" yaml:"cpu"`
	GPU         *GPUBlock    `json:"gpu" yaml:"gpu"`
	BuildStage  []BuildStage `json:"build_stage" yaml:"build_stage"`
}

type CPU struct {
	Detected   string   `json:"detected" yaml:"detected"`
	Preferred  string   `json:"preferred" yaml:"preferred"`
	Alternates []string `json:"alternates,omitempty" yaml:"alternates,omitempty"`
}

type GPUBlock struct {
	Vendor              string `json:"vendor" yaml:"vendor"`
	DriverVersion       string `json:"driver_version" yaml:"driver_version"`
	ToolkitCeiling      string `json:"toolkit_ceiling" yaml:"toolkit_ceiling"`
	ArchTarget          string `json:"arch_target" yaml:"arch_target"`
	CUDACompatAvailable bool   `json:"cuda_compat_available,omitempty" yaml:"cuda_compat_available,omitempty"`
}

type BuildStage struct {
	Path            string   `json:"path" yaml:"path"`
	Visibility      string   `json:"visibility" yaml:"visibility"`
	Writable        bool     `json:"writable" yaml:"writable"`
	FreeGB          *int     `json:"free_gb,omitempty" yaml:"free_gb,omitempty"`
	FreeInodes      *int     `json:"free_inodes,omitempty" yaml:"free_inodes,omitempty"`
	MountOpts       []string `json:"mount_opts,omitempty" yaml:"mount_opts,omitempty"`
	ThroughputClass string   `json:"throughput_class,omitempty" yaml:"throughput_class,omitempty"`
}
