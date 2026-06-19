package probes

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	inspectorhints "github.com/ray12514/cluster-inspector/internal/hints"
	"github.com/ray12514/cluster-inspector/internal/model"
)

// GPUResult contains GPU facts for the local host.
type GPUResult struct {
	GPU      *model.GPUBlock
	Evidence map[string]model.Evidence
}

// GPUToolkitModulesResult contains system-wide standalone GPU toolkit modules.
type GPUToolkitModulesResult struct {
	Toolkits *model.GPUToolkitModules
	Evidence map[string]model.Evidence
}

// ProbeGPU discovers GPU vendor, driver version, toolkit ceiling, and
// arch label. Returns nil GPU block on hosts without a GPU.
func ProbeGPU() GPUResult {
	result := GPUResult{Evidence: map[string]model.Evidence{}}
	if gpu := probeNVIDIAGPU(result.Evidence); gpu != nil {
		result.GPU = gpu
		return result
	}
	if gpu := probeAMDGPU(result.Evidence); gpu != nil {
		result.GPU = gpu
		return result
	}
	appendEvidence(result.Evidence, "gpu", evidence(model.ConfidenceProbed, "no NVIDIA/AMD GPU command evidence found"))
	return result
}

// ProbeGPUToolkitModules discovers standalone ROCm/CUDA/NVHPC toolkit facts.
func ProbeGPUToolkitModules() GPUToolkitModulesResult {
	return ProbeGPUToolkitModulesWithModules(nil, nil)
}

// ProbeGPUToolkitModulesWithModules discovers standalone GPU toolkit facts
// from the active environment and verified module candidates.
func ProbeGPUToolkitModulesWithModules(candidates []ModuleCandidate, hints *inspectorhints.Hints) GPUToolkitModulesResult {
	result := GPUToolkitModulesResult{
		Toolkits: &model.GPUToolkitModules{},
		Evidence: map[string]model.Evidence{},
	}

	if rocm := probeROCmToolkit(result.Evidence); rocm != nil {
		result.Toolkits.ROCm = rocm
	}
	if cuda := probeCUDAToolkit(result.Evidence); cuda != nil {
		result.Toolkits.CUDAToolkit = cuda
	}
	if nvhpc := probeNvhpcToolkit(result.Evidence); nvhpc != nil {
		result.Toolkits.NVHPC = nvhpc
	}
	applyVerifiedGPUToolkitModules(result.Toolkits, candidates, hints, result.Evidence)
	applyGPUToolkitExtras(result.Toolkits, hints, result.Evidence)
	if result.Toolkits.ROCm == nil && result.Toolkits.CUDAToolkit == nil && result.Toolkits.NVHPC == nil {
		appendEvidence(result.Evidence, "gpu_toolkit_modules", evidence(model.ConfidenceUnknown, "no standalone GPU toolkit evidence found"))
		return GPUToolkitModulesResult{Evidence: result.Evidence}
	}
	return result
}

func probeNVIDIAGPU(evidenceMap map[string]model.Evidence) *model.GPUBlock {
	if commandPath("nvidia-smi") == "" {
		return nil
	}
	if out, err := run("nvidia-smi", "-L"); err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	driver := ""
	if out, err := run("nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader"); err == nil {
		driver = strings.Fields(strings.TrimSpace(out))[0]
	}
	arch := ""
	if out, err := run("nvidia-smi", "--query-gpu=compute_cap", "--format=csv,noheader"); err == nil {
		arch = nvidiaArch(out)
	}
	if driver == "" || arch == "" {
		return nil
	}
	appendEvidence(evidenceMap, "gpu.vendor", evidence(model.ConfidenceProbed, "nvidia-smi"))
	return &model.GPUBlock{
		Vendor:         "nvidia",
		DriverVersion:  driver,
		ToolkitCeiling: "unknown",
		ArchTarget:     arch,
	}
}

func probeAMDGPU(evidenceMap map[string]model.Evidence) *model.GPUBlock {
	if commandPath("rocminfo") == "" && commandPath("rocm-smi") == "" {
		return nil
	}
	arch := ""
	if commandPath("rocminfo") != "" {
		if out, err := run("rocminfo"); err == nil {
			arch = amdArch(out)
		}
	}
	driver := ""
	if commandPath("rocm-smi") != "" {
		if out, err := run("rocm-smi", "--showdriverversion"); err == nil {
			driver = firstVersion(out)
		}
	}
	if driver == "" && commandPath("modinfo") != "" {
		if out, err := run("modinfo", "amdgpu"); err == nil {
			driver = firstVersion(out)
		}
	}
	if arch == "" || driver == "" {
		return nil
	}
	appendEvidence(evidenceMap, "gpu.vendor", evidence(model.ConfidenceProbed, "rocminfo/rocm-smi"))
	return &model.GPUBlock{
		Vendor:         "amd",
		DriverVersion:  driver,
		ToolkitCeiling: "unknown",
		ArchTarget:     arch,
	}
}

func nvidiaArch(out string) string {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) == 0 {
		return ""
	}
	capability := strings.Trim(fields[0], ",")
	major, minor, ok := strings.Cut(capability, ".")
	if !ok {
		return ""
	}
	return "sm_" + major + minor
}

func amdArch(out string) string {
	re := regexp.MustCompile(`gfx[0-9a-fA-F]+`)
	return re.FindString(out)
}

func probeROCmToolkit(evidenceMap map[string]model.Evidence) *model.ROCmToolkitModule {
	prefix := firstExistingDir(envOrEmpty("ROCM_PATH"), "/opt/rocm")
	if prefix == "" && commandPath("hipcc") != "" {
		prefix = prefixFromCommand(commandPath("hipcc"))
	}
	if prefix == "" {
		return nil
	}
	version := ""
	if out, err := run(filepathJoin(prefix, "bin", "hipcc"), "--version"); err == nil {
		version = firstVersion(out)
	}
	if version == "" {
		version = firstVersion(prefix)
	}
	if version == "" {
		version = "unknown"
	}
	appendEvidence(evidenceMap, "gpu_toolkit_modules.rocm", evidence(model.ConfidenceProbed, "ROCM_PATH/hipcc"))
	return &model.ROCmToolkitModule{
		Version:         version,
		Module:          "rocm/" + version,
		Prefix:          prefix,
		SpackComponents: rocmSpackComponents(prefix),
	}
}

func probeCUDAToolkit(evidenceMap map[string]model.Evidence) *model.CUDAToolkitModule {
	prefix := firstExistingDir(envOrEmpty("CUDA_HOME"), envOrEmpty("CUDA_PATH"), "/usr/local/cuda")
	if prefix == "" && commandPath("nvcc") != "" {
		prefix = prefixFromCommand(commandPath("nvcc"))
	}
	if prefix == "" {
		return nil
	}
	version := ""
	if out, err := run(filepathJoin(prefix, "bin", "nvcc"), "--version"); err == nil {
		version = firstVersion(out)
	}
	if version == "" {
		version = firstVersion(prefix)
	}
	if version == "" {
		version = "unknown"
	}
	appendEvidence(evidenceMap, "gpu_toolkit_modules.cudatoolkit", evidence(model.ConfidenceProbed, "CUDA_HOME/nvcc"))
	return &model.CUDAToolkitModule{
		Version: version,
		Module:  "cudatoolkit/" + version,
		Prefix:  prefix,
	}
}

func probeNvhpcToolkit(evidenceMap map[string]model.Evidence) *model.NvhpcToolkitModule {
	prefix := firstExistingDir(envOrEmpty("NVHPC_ROOT"), "/opt/nvidia/hpc_sdk")
	if prefix == "" && commandPath("nvc") != "" {
		prefix = prefixFromCommand(commandPath("nvc"))
	}
	if prefix == "" {
		return nil
	}
	version := firstVersion(prefix)
	if version == "" && commandPath("nvc") != "" {
		if out, err := run("nvc", "--version"); err == nil {
			version = firstVersion(out)
		}
	}
	if version == "" {
		version = "unknown"
	}
	appendEvidence(evidenceMap, "gpu_toolkit_modules.nvhpc", evidence(model.ConfidenceProbed, "NVHPC_ROOT/nvc"))
	return &model.NvhpcToolkitModule{
		Version: version,
		Module:  "nvhpc/" + version,
		Prefix:  prefix,
	}
}

func envOrEmpty(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func filepathJoin(elem ...string) string {
	for _, part := range elem {
		if part == "" {
			return ""
		}
	}
	return filepath.Join(elem...)
}

func applyVerifiedGPUToolkitModules(toolkits *model.GPUToolkitModules, candidates []ModuleCandidate, hints *inspectorhints.Hints, evidenceMap map[string]model.Evidence) {
	moduleHints := inspectorhints.ModuleHints{}
	if hints != nil {
		moduleHints = hints.GPUToolkits
	}
	accepted := applyModulePolicy(candidateNamesByCategory(candidates, "gpu_toolkit"), moduleHints, nil, evidenceMap, "gpu_toolkit_modules.module_hints")
	for _, module := range accepted {
		name := gpuToolkitNameFromModule(module)
		if name == "" {
			continue
		}
		verification, err := verifyModules([]string{module})
		if err != nil {
			appendVerificationFailure(evidenceMap, "gpu_toolkit_modules.verify_failed."+module, []string{module}, err)
			continue
		}
		switch name {
		case "rocm":
			rocm, ok := rocmToolkitFromVerification(module, verification)
			if !ok {
				appendEvidence(evidenceMap, "gpu_toolkit_modules.verify_failed."+module, evidence(model.ConfidenceUnknown, "module loaded but ROCm prefix unavailable"))
				continue
			}
			toolkits.ROCm = rocm
			appendEvidence(evidenceMap, "gpu_toolkit_modules.rocm", evidence(model.ConfidenceProbed, "clean-shell module verification"))
		case "cudatoolkit":
			cuda, ok := cudaToolkitFromVerification(module, verification)
			if !ok {
				appendEvidence(evidenceMap, "gpu_toolkit_modules.verify_failed."+module, evidence(model.ConfidenceUnknown, "module loaded but CUDA prefix unavailable"))
				continue
			}
			toolkits.CUDAToolkit = cuda
			appendEvidence(evidenceMap, "gpu_toolkit_modules.cudatoolkit", evidence(model.ConfidenceProbed, "clean-shell module verification"))
		case "nvhpc":
			nvhpc, ok := nvhpcToolkitFromVerification(module, verification)
			if !ok {
				appendEvidence(evidenceMap, "gpu_toolkit_modules.verify_failed."+module, evidence(model.ConfidenceUnknown, "module loaded but NVHPC prefix unavailable"))
				continue
			}
			toolkits.NVHPC = nvhpc
			appendEvidence(evidenceMap, "gpu_toolkit_modules.nvhpc", evidence(model.ConfidenceProbed, "clean-shell module verification"))
		}
	}
}

func applyGPUToolkitExtras(toolkits *model.GPUToolkitModules, hints *inspectorhints.Hints, evidenceMap map[string]model.Evidence) {
	if hints == nil {
		return
	}
	for _, extra := range hints.Extras.GPUToolkits {
		switch strings.ToLower(extra.Name) {
		case "rocm":
			components := make([]model.SpackComponent, 0, len(extra.SpackComponents))
			for _, component := range extra.SpackComponents {
				components = append(components, model.SpackComponent{Package: component.Package, Prefix: component.Prefix})
			}
			if len(components) == 0 {
				components = rocmSpackComponents(extra.Prefix)
			}
			toolkits.ROCm = &model.ROCmToolkitModule{Version: extra.Version, Module: extra.Module, Prefix: extra.Prefix, SpackComponents: components}
			appendEvidence(evidenceMap, "gpu_toolkit_modules.rocm", evidence(model.ConfidenceInferred, "inspector-hints extras"))
		case "cuda", "cudatoolkit":
			toolkits.CUDAToolkit = &model.CUDAToolkitModule{Version: extra.Version, Module: extra.Module, Prefix: extra.Prefix}
			appendEvidence(evidenceMap, "gpu_toolkit_modules.cudatoolkit", evidence(model.ConfidenceInferred, "inspector-hints extras"))
		case "nvhpc":
			toolkits.NVHPC = &model.NvhpcToolkitModule{Version: extra.Version, Module: extra.Module, Prefix: extra.Prefix}
			appendEvidence(evidenceMap, "gpu_toolkit_modules.nvhpc", evidence(model.ConfidenceInferred, "inspector-hints extras"))
		}
	}
}

func gpuToolkitNameFromModule(module string) string {
	lower := strings.ToLower(module)
	switch {
	case strings.HasPrefix(lower, "rocm/"):
		return "rocm"
	case strings.HasPrefix(lower, "cuda/") || strings.HasPrefix(lower, "cudatoolkit/"):
		return "cudatoolkit"
	case strings.HasPrefix(lower, "nvhpc/"):
		return "nvhpc"
	default:
		return ""
	}
}

func rocmToolkitFromVerification(module string, verification moduleVerification) (*model.ROCmToolkitModule, bool) {
	prefix := firstNonEmptyString(verification.Env["ROCM_PATH"], prefixFromCommand(verification.Commands["hipcc"]))
	if prefix == "" {
		return nil, false
	}
	version := moduleVersion(module)
	if version == "" {
		version = firstVersion(prefix)
	}
	if version == "" {
		version = "unknown"
	}
	return &model.ROCmToolkitModule{
		Version:         version,
		Module:          module,
		Prefix:          prefix,
		SpackComponents: rocmSpackComponents(prefix),
	}, true
}

func cudaToolkitFromVerification(module string, verification moduleVerification) (*model.CUDAToolkitModule, bool) {
	prefix := firstNonEmptyString(verification.Env["CUDA_HOME"], verification.Env["CUDA_PATH"], prefixFromCommand(verification.Commands["nvcc"]))
	if prefix == "" {
		return nil, false
	}
	version := moduleVersion(module)
	if version == "" {
		version = firstVersion(prefix)
	}
	if version == "" {
		version = "unknown"
	}
	return &model.CUDAToolkitModule{Version: version, Module: module, Prefix: prefix}, true
}

func nvhpcToolkitFromVerification(module string, verification moduleVerification) (*model.NvhpcToolkitModule, bool) {
	prefix := firstNonEmptyString(verification.Env["NVHPC_ROOT"], prefixFromCommand(verification.Commands["nvc"]))
	if prefix == "" {
		return nil, false
	}
	version := moduleVersion(module)
	if version == "" {
		version = firstVersion(prefix)
	}
	if version == "" {
		version = "unknown"
	}
	return &model.NvhpcToolkitModule{Version: version, Module: module, Prefix: prefix}, true
}

func rocmSpackComponents(prefix string) []model.SpackComponent {
	return []model.SpackComponent{
		{Package: "hip", Prefix: filepath.Join(prefix, "hip")},
		{Package: "hsa-rocr-dev", Prefix: prefix},
		{Package: "comgr", Prefix: prefix},
		{Package: "rocblas", Prefix: prefix},
		{Package: "hipblas", Prefix: prefix},
		{Package: "hipsparse", Prefix: prefix},
		{Package: "rocprim", Prefix: filepath.Join(prefix, "rocprim")},
		{Package: "llvm-amdgpu", Prefix: prefix},
	}
}
