package probes

import (
	"os"
	"path/filepath"

	"github.com/ray12514/cluster-inspector/internal/model"
)

// CrayPEResult contains Cray PE inventory when present.
type CrayPEResult struct {
	VendorCray *model.VendorCray
	Evidence   map[string]model.Evidence
}

// ProbeCrayPE detects Cray Programming Environment presence and inventory:
// PE version, CCE / Cray GCC / AOCC / Intel / ROCmCC / NVHPC compilers, Cray MPICH
// flavors (per-compiler prefixes), libsci.
//
// TODO: Phase 4 — add module candidate enumeration and clean-shell
// verification for PrgEnv and cray-mpich module combinations.
func ProbeCrayPE() CrayPEResult {
	result := CrayPEResult{Evidence: map[string]model.Evidence{}}
	if !detectCrayEvidence() {
		appendEvidence(result.Evidence, "vendor_cray", evidence(model.ConfidenceProbed, "no Cray PE evidence"))
		return result
	}

	peVersion := os.Getenv("CRAYPE_VERSION")
	if peVersion == "" {
		peVersion = latestChildVersion("/opt/cray/pe")
	}
	if peVersion == "" {
		peVersion = "unknown"
		appendEvidence(result.Evidence, "vendor_cray.pe_version", evidence(model.ConfidenceUnknown, "Cray PE present; version unavailable"))
	} else {
		appendEvidence(result.Evidence, "vendor_cray.pe_version", evidence(model.ConfidenceProbed, "CRAYPE_VERSION or /opt/cray/pe"))
	}

	vendor := &model.VendorCray{PEVersion: peVersion}
	vendor.CCE = crayCompiler("cce", os.Getenv("CRAY_PE_CCE_PREFIX"), "/opt/cray/pe/cce", []string{"PrgEnv-cray"})
	vendor.GCC = crayCompiler("gcc", os.Getenv("GCC_PATH"), "/opt/cray/pe/gcc", []string{"PrgEnv-gnu"})
	vendor.AOCC = crayAOCCCompiler()
	vendor.Intel = crayIntelCompiler()
	vendor.ROCmCC = crayCompiler("rocmcc", os.Getenv("ROCM_PATH"), "/opt/rocm", []string{"PrgEnv-amd"})
	vendor.NVHPC = crayCompiler("nvhpc", os.Getenv("NVHPC_ROOT"), "/opt/nvidia/hpc_sdk", []string{"PrgEnv-nvidia"})
	vendor.CrayMPICH = crayMPICH()
	vendor.LibSci = crayLibSci()

	appendCrayCompilerEvidence(result.Evidence, "cce", vendor.CCE)
	appendCrayCompilerEvidence(result.Evidence, "gcc", vendor.GCC)
	appendCrayCompilerEvidence(result.Evidence, "aocc", vendor.AOCC)
	appendCrayCompilerEvidence(result.Evidence, "intel", vendor.Intel)
	appendCrayCompilerEvidence(result.Evidence, "rocmcc", vendor.ROCmCC)
	appendCrayCompilerEvidence(result.Evidence, "nvhpc", vendor.NVHPC)
	if vendor.CrayMPICH != nil {
		appendEvidence(result.Evidence, "vendor_cray.cray_mpich", evidence(model.ConfidenceProbed, "CRAY_MPICH_VERSION/MPICH_DIR or /opt/cray/pe/mpich"))
	}
	result.VendorCray = vendor
	return result
}

func crayAOCCCompiler() *model.CrayCompilerBlock {
	return crayCompiler(
		"aocc",
		firstNonEmptyEnv("AOCC_HOME", "AOCC_ROOT", "AOMP"),
		firstExistingDir("/opt/cray/pe/aocc", "/opt/AMD/aocc-compiler", "/opt/AMD"),
		[]string{"PrgEnv-aocc"},
	)
}

func crayIntelCompiler() *model.CrayCompilerBlock {
	return crayCompiler(
		"intel",
		firstNonEmptyEnv("INTEL_PATH", "INTEL_HOME", "ONEAPI_ROOT", "CMPLR_ROOT"),
		firstExistingDir("/opt/cray/pe/intel", "/opt/intel/oneapi/compiler", "/opt/intel"),
		[]string{"PrgEnv-intel"},
	)
}

func appendCrayCompilerEvidence(evidenceMap map[string]model.Evidence, name string, compiler *model.CrayCompilerBlock) {
	if compiler == nil {
		return
	}
	appendEvidence(evidenceMap, "vendor_cray."+name, evidence(model.ConfidenceProbed, compiler.Prefix))
}

func crayCompiler(name, envPrefix, fallbackRoot string, modules []string) *model.CrayCompilerBlock {
	prefix := envPrefix
	if prefix == "" && isDir(fallbackRoot) {
		version := latestChildVersion(fallbackRoot)
		if version != "" {
			prefix = filepath.Join(fallbackRoot, version)
		}
	}
	if prefix == "" {
		return nil
	}
	version := firstVersion(prefix)
	if version == "" {
		version = "unknown"
	}
	if len(modules) == 1 && version != "unknown" {
		modules = append(modules, name+"/"+version)
	}
	return &model.CrayCompilerBlock{
		Version: version,
		Prefix:  prefix,
		Modules: modules,
	}
}

func crayMPICH() *model.CrayMPICHBlock {
	version := os.Getenv("CRAY_MPICH_VERSION")
	prefix := os.Getenv("MPICH_DIR")
	if version == "" && prefix != "" {
		version = firstVersion(prefix)
	}
	if prefix == "" && isDir("/opt/cray/pe/mpich") {
		version = latestChildVersion("/opt/cray/pe/mpich")
		if version != "" {
			prefix = filepath.Join("/opt/cray/pe/mpich", version)
		}
	}
	if version == "" || prefix == "" {
		return nil
	}
	return &model.CrayMPICHBlock{
		Version: version,
		Flavors: map[string]model.CrayMPICHFlavor{
			crayCompilerFlavor(): {
				Prefix:  prefix,
				Modules: []string{"cray-mpich/" + version},
			},
		},
	}
}

func crayCompilerFlavor() string {
	switch os.Getenv("PE_ENV") {
	case "GNU":
		return "gcc"
	case "CRAY":
		return "cce"
	case "AOCC":
		return "aocc"
	case "INTEL":
		return "intel"
	case "AMD":
		return "rocmcc"
	case "NVIDIA":
		return "nvhpc"
	default:
		return "unknown"
	}
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}

func crayLibSci() *model.CrayLibSciBlock {
	prefix := os.Getenv("CRAY_LIBSCI_PREFIX_DIR")
	if prefix == "" && isDir("/opt/cray/pe/libsci") {
		version := latestChildVersion("/opt/cray/pe/libsci")
		if version != "" {
			prefix = filepath.Join("/opt/cray/pe/libsci", version)
		}
	}
	if prefix == "" {
		return nil
	}
	version := os.Getenv("CRAY_LIBSCI_VERSION")
	if version == "" {
		version = firstVersion(prefix)
	}
	return &model.CrayLibSciBlock{Version: version, Prefix: prefix}
}
