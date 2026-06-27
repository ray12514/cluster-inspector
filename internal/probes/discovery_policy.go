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
	VersionEnv     []string `yaml:"version_env"`
	Roots          []string `yaml:"roots"`
	ModuleSegments []string `yaml:"module_segments"`
	ModulePrefixes []string `yaml:"module_name_prefixes"`
	PlatformOwned  bool     `yaml:"platform_owned"`
}

type mpiPolicy struct {
	Name            string     `yaml:"name"`
	ModuleSegments  []string   `yaml:"module_segments"`
	Env             []string   `yaml:"env"`
	VersionEnv      []string   `yaml:"version_env"`
	VersionCommands [][]string `yaml:"version_commands"`
	PlatformOwned   bool       `yaml:"platform_owned"`
}

type gpuToolkitPolicy struct {
	Name                string                 `yaml:"name"`
	ModuleSegments      []string               `yaml:"module_segments"`
	Env                 []string               `yaml:"env"`
	Commands            []string               `yaml:"commands"`
	Roots               []string               `yaml:"roots"`
	ModuleTemplate      string                 `yaml:"module_template"`
	ComponentCandidates []spackComponentPolicy `yaml:"component_candidates"`
}

type spackComponentPolicy struct {
	Package      string   `yaml:"package"`
	PrefixSuffix string   `yaml:"prefix_suffix"`
	Required     bool     `yaml:"required"`
	ProbePaths   []string `yaml:"probe_paths"`
}

type systemExternalsPolicy struct {
	DefaultFocus       []string               `yaml:"default_focus"`
	ExternalCandidates []systemExternalPolicy `yaml:"external_candidates"`
}

type systemExternalPolicy struct {
	Name           string   `yaml:"name"`
	ProviderFamily string   `yaml:"provider_family"`
	Env            []string `yaml:"env"`
	VersionEnv     []string `yaml:"version_env"`
	Roots          []string `yaml:"roots"`
	ModuleTemplate string   `yaml:"module_template"`
}

type filesystemPolicy struct {
	SharedProbeRoots  []string `yaml:"shared_probe_roots"`
	ScratchProbeRoots []string `yaml:"scratch_probe_roots"`
}

type fabricPolicy struct {
	CXIDevicePaths      []string                `yaml:"cxi_device_paths"`
	CXIUserspaceRoots   []string                `yaml:"cxi_userspace_roots"`
	InfiniBandClassPath string                  `yaml:"infiniband_class_path"`
	UserspaceCandidates []fabricUserspacePolicy `yaml:"userspace_candidates"`
}

type fabricUserspacePolicy struct {
	Name            string     `yaml:"name"`
	Commands        []string   `yaml:"commands"`
	Env             []string   `yaml:"env"`
	Roots           []string   `yaml:"roots"`
	VersionCommands [][]string `yaml:"version_commands"`
	ModuleSegments  []string   `yaml:"module_segments"`
}

type platformPolicy struct {
	PlatformFamily             string                       `yaml:"platform_family"`
	EvidencePaths              []string                     `yaml:"evidence_paths"`
	EvidenceEnv                []string                     `yaml:"evidence_env"`
	EvidenceLoadedModuleTokens []string                     `yaml:"evidence_loaded_module_segments"`
	OwnedPrefixes              []string                     `yaml:"owned_prefixes"`
	ModuleCategories           []string                     `yaml:"module_categories"`
	ModuleSegments             []string                     `yaml:"module_segments"`
	ModulePrefixes             []string                     `yaml:"module_name_prefixes"`
	PERoot                     string                       `yaml:"pe_root"`
	ProviderEnvMappings        map[string]map[string]string `yaml:"provider_env_mappings"`
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

func compilerVersionEnvKeys(name string) []string {
	if item, ok := compilerPolicyByName(name); ok {
		return item.VersionEnv
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

func mpiVersionEnvKeys(name string) []string {
	if item, ok := mpiPolicyByName(name); ok {
		return item.VersionEnv
	}
	return nil
}

func systemExternalPolicyByName(name string) (systemExternalPolicy, bool) {
	for _, item := range policy().SystemExternals.ExternalCandidates {
		if item.Name == name {
			return item, true
		}
	}
	return systemExternalPolicy{}, false
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

func platformPolicyByFamily(family string) (platformPolicy, bool) {
	item, ok := policy().Platforms[family]
	return item, ok
}

func platformPolicies() []platformPolicy {
	out := make([]platformPolicy, 0, len(policy().Platforms))
	for _, item := range policy().Platforms {
		out = append(out, item)
	}
	return out
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

func platformProviderFromEnv(platform platformPolicy, envName string, envValue string) string {
	if platform.ProviderEnvMappings == nil {
		return ""
	}
	values := platform.ProviderEnvMappings[envName]
	if values == nil {
		return ""
	}
	return values[envValue]
}
