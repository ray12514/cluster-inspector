package probes

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
		Version: version,
		Module:  "rocm/" + version,
		Prefix:  prefix,
		SpackComponents: []model.SpackComponent{
			{Package: "hip", Prefix: prefix},
			{Package: "hsa-rocr-dev", Prefix: prefix},
			{Package: "llvm-amdgpu", Prefix: prefix},
		},
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
