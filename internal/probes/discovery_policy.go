package probes

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ray12514/cluster-inspector/internal/resources"
	"gopkg.in/yaml.v3"
)

type discoveryPolicy struct {
	SchemaVersion   int                       `yaml:"schema_version"`
	Compilers       []compilerPolicy          `yaml:"compilers"`
	MPI             []mpiPolicy               `yaml:"mpi"`
	GPUToolkits     []gpuToolkitPolicy        `yaml:"gpu_toolkits"`
	SystemExternals systemExternalsPolicy     `yaml:"system_externals"`
	Filesystem      filesystemPolicy          `yaml:"filesystem"`
	Fabric          fabricPolicy              `yaml:"fabric"`
	Platforms       map[string]platformPolicy `yaml:"platforms"`
}

type compilerPolicy struct {
	Name           string   `yaml:"name"`
	CC             []string `yaml:"cc"`
	Cxx            []string `yaml:"cxx"`
	Fortran        []string `yaml:"fortran"`
	Env            []string `yaml:"env"`
	Roots          []string `yaml:"roots"`
	ModuleSegments []string `yaml:"module_segments"`
	ModulePrefixes []string `yaml:"module_prefixes"`
	PlatformOwned  bool     `yaml:"platform_owned"`
}

type mpiPolicy struct {
	Name            string     `yaml:"name"`
	ModuleSegments  []string   `yaml:"module_segments"`
	Env             []string   `yaml:"env"`
	VersionCommands [][]string `yaml:"version_commands"`
	PlatformOwned   bool       `yaml:"platform_owned"`
}

type gpuToolkitPolicy struct {
	Name            string                 `yaml:"name"`
	ModuleSegments  []string               `yaml:"module_segments"`
	Env             []string               `yaml:"env"`
	Commands        []string               `yaml:"commands"`
	Roots           []string               `yaml:"roots"`
	ModuleTemplate  string                 `yaml:"module_template"`
	SpackComponents []spackComponentPolicy `yaml:"spack_components"`
}

type spackComponentPolicy struct {
	Package      string `yaml:"package"`
	PrefixSuffix string `yaml:"prefix_suffix"`
}

type systemExternalsPolicy struct {
	DefaultFocus []string `yaml:"default_focus"`
}

type filesystemPolicy struct {
	InstallTreePaths []string `yaml:"install_tree_paths"`
	SharedRoots      []string `yaml:"shared_roots"`
	ScratchRoots     []string `yaml:"scratch_roots"`
}

type fabricPolicy struct {
	CXIDevicePaths      []string `yaml:"cxi_device_paths"`
	CXIUserspaceRoots   []string `yaml:"cxi_userspace_roots"`
	InfiniBandClassPath string   `yaml:"infiniband_class_path"`
}

type platformPolicy struct {
	PlatformFamily             string   `yaml:"platform_family"`
	EvidencePaths              []string `yaml:"evidence_paths"`
	EvidenceEnv                []string `yaml:"evidence_env"`
	EvidenceLoadedModuleTokens []string `yaml:"evidence_loaded_module_segments"`
	OwnedPrefixes              []string `yaml:"owned_prefixes"`
	ModuleCategories           []string `yaml:"module_categories"`
	ModuleSegments             []string `yaml:"module_segments"`
	ModulePrefixes             []string `yaml:"module_prefixes"`
	PERoot                     string   `yaml:"pe_root"`
}

var (
	policyOnce sync.Once
	policyData discoveryPolicy
)

func policy() discoveryPolicy {
	policyOnce.Do(func() {
		if err := yaml.Unmarshal(resources.DiscoveryPolicy, &policyData); err != nil {
			panic(fmt.Sprintf("invalid embedded discovery policy: %v", err))
		}
		if policyData.SchemaVersion != 1 {
			panic(fmt.Sprintf("unsupported embedded discovery policy schema_version %d", policyData.SchemaVersion))
		}
	})
	return policyData
}

func compilerPolicyByName(name string) (compilerPolicy, bool) {
	for _, item := range policy().Compilers {
		if item.Name == name {
			return item, true
		}
	}
	return compilerPolicy{}, false
}

func compilerPolicyModulePrefixes(name string) []string {
	if item, ok := compilerPolicyByName(name); ok {
		return item.ModulePrefixes
	}
	return nil
}

func compilerPolicyRoots(name string) []string {
	if item, ok := compilerPolicyByName(name); ok {
		return item.Roots
	}
	return nil
}

func mpiPolicyByName(name string) (mpiPolicy, bool) {
	for _, item := range policy().MPI {
		if item.Name == name {
			return item, true
		}
	}
	return mpiPolicy{}, false
}

func gpuToolkitPolicyByName(name string) (gpuToolkitPolicy, bool) {
	for _, item := range policy().GPUToolkits {
		if item.Name == name {
			return item, true
		}
	}
	return gpuToolkitPolicy{}, false
}

func crayPEPolicy() platformPolicy {
	return policy().Platforms["cray-pe"]
}

func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func platformOwnsPrefix(prefix string) bool {
	for _, platform := range policy().Platforms {
		if hasAnyPrefix(prefix, platform.OwnedPrefixes) {
			return true
		}
	}
	return false
}

func providerFamilyFromPrefix(prefix string) string {
	if strings.HasPrefix(prefix, "/usr") || strings.HasPrefix(prefix, "/bin") {
		return "system"
	}
	return "site"
}

func firstExistingPolicyRoot(roots []string) string {
	for _, root := range roots {
		if isDir(root) {
			return root
		}
	}
	return ""
}

func componentPrefix(prefix string, component spackComponentPolicy) string {
	if component.PrefixSuffix == "" {
		return prefix
	}
	return filepath.Join(prefix, component.PrefixSuffix)
}
