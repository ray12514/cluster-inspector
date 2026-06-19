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
	VendorCray        *VendorCray         `json:"vendor_cray" yaml:"vendor_cray"`
	CompilersExternal []CompilerExternal  `json:"compilers_external,omitempty" yaml:"compilers_external,omitempty"`
	MPI               []MPIExternal       `json:"mpi,omitempty" yaml:"mpi,omitempty"`
	GPUToolkitModules *GPUToolkitModules  `json:"gpu_toolkit_modules,omitempty" yaml:"gpu_toolkit_modules,omitempty"`
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

type CompilerExternal struct {
	Name      string   `json:"name" yaml:"name"`
	Version   string   `json:"version" yaml:"version"`
	Prefix    string   `json:"prefix" yaml:"prefix"`
	Modules   []string `json:"modules,omitempty" yaml:"modules,omitempty"`
	Languages []string `json:"languages" yaml:"languages"`
}

type MPIExternal struct {
	Name       string   `json:"name" yaml:"name"`
	Provenance string   `json:"provenance" yaml:"provenance"`
	Version    string   `json:"version" yaml:"version"`
	Prefix     string   `json:"prefix" yaml:"prefix"`
	Compiler   string   `json:"compiler,omitempty" yaml:"compiler,omitempty"`
	Modules    []string `json:"modules,omitempty" yaml:"modules,omitempty"`
}

type VendorCray struct {
	PEVersion string             `json:"pe_version" yaml:"pe_version"`
	CCE       *CrayCompilerBlock `json:"cce,omitempty" yaml:"cce,omitempty"`
	GCC       *CrayCompilerBlock `json:"gcc,omitempty" yaml:"gcc,omitempty"`
	AOCC      *CrayCompilerBlock `json:"aocc,omitempty" yaml:"aocc,omitempty"`
	Intel     *CrayCompilerBlock `json:"intel,omitempty" yaml:"intel,omitempty"`
	ROCmCC    *CrayCompilerBlock `json:"rocmcc,omitempty" yaml:"rocmcc,omitempty"`
	NVHPC     *CrayCompilerBlock `json:"nvhpc,omitempty" yaml:"nvhpc,omitempty"`
	CrayMPICH *CrayMPICHBlock    `json:"cray_mpich,omitempty" yaml:"cray_mpich,omitempty"`
	LibSci    *CrayLibSciBlock   `json:"libsci,omitempty" yaml:"libsci,omitempty"`
}

type CrayCompilerBlock struct {
	Version string   `json:"version" yaml:"version"`
	Prefix  string   `json:"prefix" yaml:"prefix"`
	Modules []string `json:"modules" yaml:"modules"`
}

type CrayMPICHBlock struct {
	Version string                     `json:"version" yaml:"version"`
	Flavors map[string]CrayMPICHFlavor `json:"flavors" yaml:"flavors"`
}

type CrayMPICHFlavor struct {
	Prefix  string   `json:"prefix" yaml:"prefix"`
	Modules []string `json:"modules" yaml:"modules"`
}

type CrayLibSciBlock struct {
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	Prefix  string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
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
