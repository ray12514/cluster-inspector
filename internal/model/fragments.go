package model

// SystemFragment is the output of `cluster-inspector probe-system`. It
// contains all system-wide facts (everything except per-node-type entries).
type SystemFragment struct {
	SchemaVersion     int                 `json:"schema_version" yaml:"schema_version"`
	System            System              `json:"system" yaml:"system"`
	OS                OS                  `json:"os" yaml:"os"`
	Fabric            Fabric              `json:"fabric" yaml:"fabric"`
	ModulesSystem     ModulesSystem       `json:"modules_system" yaml:"modules_system"`
	VendorCray        *VendorCray         `json:"vendor_cray" yaml:"vendor_cray"`
	CompilersExternal []CompilerExternal  `json:"compilers_external,omitempty" yaml:"compilers_external,omitempty"`
	MPI               []MPIExternal       `json:"mpi,omitempty" yaml:"mpi,omitempty"`
	GPUToolkitModules *GPUToolkitModules  `json:"gpu_toolkit_modules,omitempty" yaml:"gpu_toolkit_modules,omitempty"`
	SystemExternals   []SystemExternal    `json:"system_externals,omitempty" yaml:"system_externals,omitempty"`
	Filesystem        Filesystem          `json:"filesystem" yaml:"filesystem"`
	ModulePaths       []string            `json:"module_paths,omitempty" yaml:"module_paths,omitempty"`
	Evidence          map[string]Evidence `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}

// NodeFragment is the output of `cluster-inspector probe-node`. It
// contains the facts for a single node class.
type NodeFragment struct {
	Name        string              `json:"name" yaml:"name"`
	Role        string              `json:"role" yaml:"role"`
	Description string              `json:"description,omitempty" yaml:"description,omitempty"`
	CPU         CPU                 `json:"cpu" yaml:"cpu"`
	GPU         *GPUBlock           `json:"gpu" yaml:"gpu"`
	BuildStage  []BuildStage        `json:"build_stage" yaml:"build_stage"`
	Evidence    map[string]Evidence `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}
