// Package hints loads and validates inspector-hints.yaml, the committed
// override mechanism for module discovery.
//
// See stack-planning/docs/cluster_inspector_stack_profile_design_v1.md
// § Module Discovery And Hints for the full schema; the example hints
// file under that section is the reference.
package hints

// Hints mirrors inspector-hints.yaml.
type Hints struct {
	SchemaVersion   int            `yaml:"schema_version"`
	Compilers       ModuleHints    `yaml:"compilers"`
	MPI             ModuleHints    `yaml:"mpi"`
	GPUToolkits     ModuleHints    `yaml:"gpu_toolkits"`
	FabricUserspace ModuleHints    `yaml:"fabric_userspace"`
	Extras          ExplicitExtras `yaml:"extras"`
}

type ModuleHints struct {
	Include         []string `yaml:"include"`
	ExcludePatterns []string `yaml:"exclude_patterns"`
}

type ExplicitExtras struct {
	Compilers       []CompilerExtra        `yaml:"compilers"`
	MPI             []MPIExtra             `yaml:"mpi"`
	GPUToolkits     []GPUToolkitExtra      `yaml:"gpu_toolkits"`
	FabricUserspace []FabricUserspaceExtra `yaml:"fabric_userspace"`
}

type CompilerExtra struct {
	Module    string   `yaml:"module"`
	Name      string   `yaml:"name"`
	Version   string   `yaml:"version"`
	Prefix    string   `yaml:"prefix"`
	Languages []string `yaml:"languages"`
}

type MPIExtra struct {
	Module     string `yaml:"module"`
	Name       string `yaml:"name"`
	Provenance string `yaml:"provenance"`
	Version    string `yaml:"version"`
	Prefix     string `yaml:"prefix"`
	Compiler   string `yaml:"compiler"`
}

type GPUToolkitExtra struct {
	Module          string           `yaml:"module"`
	Name            string           `yaml:"name"`
	Version         string           `yaml:"version"`
	Prefix          string           `yaml:"prefix"`
	SpackComponents []SpackComponent `yaml:"spack_components"`
}

type SpackComponent struct {
	Package string `yaml:"package"`
	Prefix  string `yaml:"prefix"`
}

type FabricUserspaceExtra struct {
	Module  string `yaml:"module"`
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Prefix  string `yaml:"prefix"`
}
