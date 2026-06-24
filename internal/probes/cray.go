package probes

import (
	"os"
	"path/filepath"
	"strings"

	inspectorhints "github.com/ray12514/cluster-inspector/internal/hints"
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
func ProbeCrayPE() CrayPEResult {
	return ProbeCrayPEWithModules(nil, nil)
}

// ProbeCrayPEWithModules detects Cray PE inventory from filesystem/env
// evidence and verified module combinations.
func ProbeCrayPEWithModules(candidates []ModuleCandidate, hints *inspectorhints.Hints) CrayPEResult {
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
	applyVerifiedCrayModules(vendor, candidates, hints, result.Evidence)

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

func applyVerifiedCrayModules(vendor *model.VendorCray, candidates []ModuleCandidate, hints *inspectorhints.Hints, evidenceMap map[string]model.Evidence) {
	if vendor == nil {
		return
	}
	moduleHints := inspectorhints.ModuleHints{}
	if hints != nil {
		moduleHints = hints.Compilers
	}
	compilerCandidates := append(candidateNamesByCategory(candidates, "compiler"), candidateNamesByCategory(candidates, "cray_pe")...)
	acceptedCompilers := applyModulePolicy(compilerCandidates, moduleHints, nil, evidenceMap, "vendor_cray.compiler_module_hints")
	for _, module := range acceptedCompilers {
		flavor := crayFlavorFromModule(module)
		if flavor == "" {
			continue
		}
		moduleSet := crayCompilerModuleSet(flavor, module)
		verification, err := verifyModules(moduleSet)
		if err != nil {
			appendVerificationFailure(evidenceMap, "vendor_cray.verify_failed."+module, moduleSet, err)
			continue
		}
		compiler, ok := crayCompilerFromVerification(flavor, module, verification)
		if !ok {
			appendEvidence(evidenceMap, "vendor_cray.verify_failed."+module, evidence(model.ConfidenceUnknown, "module loaded but compiler prefix unavailable"))
			continue
		}
		setCrayCompiler(vendor, flavor, compiler)
		appendEvidence(evidenceMap, "vendor_cray."+flavor, evidence(model.ConfidenceProbed, "clean-shell module verification"))
	}
	applyVerifiedCrayMPICH(vendor, candidates, hints, evidenceMap)
}

func applyVerifiedCrayMPICH(vendor *model.VendorCray, candidates []ModuleCandidate, hints *inspectorhints.Hints, evidenceMap map[string]model.Evidence) {
	moduleHints := inspectorhints.ModuleHints{}
	if hints != nil {
		moduleHints = hints.MPI
	}
	mpichCandidates := filterModuleNames(candidateNamesByCategory(candidates, "mpi"), func(module string) bool {
		return mpiNameFromModule(module) == "cray-mpich"
	})
	accepted := applyModulePolicy(mpichCandidates, moduleHints, nil, evidenceMap, "vendor_cray.cray_mpich.module_hints")
	for _, mpichModule := range accepted {
		version := moduleVersion(mpichModule)
		if version == "" {
			version = "unknown"
		}
		if vendor.CrayMPICH == nil {
			vendor.CrayMPICH = &model.CrayMPICHBlock{Version: version, Flavors: map[string]model.CrayMPICHFlavor{}}
		}
		if vendor.CrayMPICH.Flavors == nil {
			vendor.CrayMPICH.Flavors = map[string]model.CrayMPICHFlavor{}
		}
		for _, flavor := range verifiedCrayCompilerFlavors(vendor) {
			compiler := getCrayCompiler(vendor, flavor)
			moduleSet := append([]string{}, compiler.Modules...)
			moduleSet = append(moduleSet, mpichModule)
			verification, err := verifyModules(moduleSet)
			if err != nil {
				appendVerificationFailure(evidenceMap, "vendor_cray.cray_mpich.verify_failed."+flavor, moduleSet, err)
				continue
			}
			prefix := firstNonEmptyString(verification.Env["MPICH_DIR"], prefixFromCommand(verification.Commands["mpicc"]))
			if prefix == "" {
				appendEvidence(evidenceMap, "vendor_cray.cray_mpich.verify_failed."+flavor, evidence(model.ConfidenceUnknown, "module loaded but MPICH prefix unavailable"))
				continue
			}
			vendor.CrayMPICH.Version = firstNonEmptyString(verification.Env["CRAY_MPICH_VERSION"], version)
			vendor.CrayMPICH.Flavors[flavor] = model.CrayMPICHFlavor{Prefix: prefix, Modules: []string{mpichModule}}
			appendEvidence(evidenceMap, "vendor_cray.cray_mpich."+flavor, evidence(model.ConfidenceProbed, "clean-shell module verification"))
		}
	}
}

func crayFlavorFromModule(module string) string {
	switch {
	case moduleHasSegmentPrefix(module, "prgenv-cray") || moduleHasSegment(module, "cce"):
		return "cce"
	case moduleHasSegmentPrefix(module, "prgenv-gnu") || moduleHasSegment(module, "gcc-native"):
		return "gcc"
	case moduleHasSegmentPrefix(module, "prgenv-aocc") || moduleHasSegment(module, "aocc"):
		return "aocc"
	case moduleHasSegmentPrefix(module, "prgenv-intel") || moduleHasSegment(module, "intel"):
		return "intel"
	case moduleHasSegmentPrefix(module, "prgenv-amd") || moduleHasSegment(module, "rocm", "rocmcc"):
		return "rocmcc"
	case moduleHasSegmentPrefix(module, "prgenv-nvidia") || moduleHasSegment(module, "nvhpc"):
		return "nvhpc"
	default:
		return ""
	}
}

func crayCompilerModuleSet(flavor, module string) []string {
	moduleSet := []string{}
	switch flavor {
	case "cce":
		moduleSet = append(moduleSet, "PrgEnv-cray")
	case "gcc":
		moduleSet = append(moduleSet, "PrgEnv-gnu")
	case "aocc":
		moduleSet = append(moduleSet, "PrgEnv-aocc")
	case "intel":
		moduleSet = append(moduleSet, "PrgEnv-intel")
	case "rocmcc":
		moduleSet = append(moduleSet, "PrgEnv-amd")
	case "nvhpc":
		moduleSet = append(moduleSet, "PrgEnv-nvidia")
	}
	if !strings.HasPrefix(strings.ToLower(module), "prgenv-") {
		moduleSet = append(moduleSet, module)
	}
	return cleanModuleList(moduleSet)
}

func crayCompilerFromVerification(flavor, module string, verification moduleVerification) (*model.CrayCompilerBlock, bool) {
	name := crayCompilerName(flavor)
	prefix := compilerPrefixFromVerification(name, verification)
	if prefix == "" && flavor == "cce" {
		prefix = verification.Env["CRAY_PE_CCE_PREFIX"]
	}
	if prefix == "" {
		return nil, false
	}
	version := firstNonEmptyString(crayCompilerVersionFromEnv(flavor, verification), moduleVersion(module), firstVersion(prefix))
	if version == "" {
		version = "unknown"
	}
	return &model.CrayCompilerBlock{Version: version, Prefix: prefix, Modules: verification.Modules}, true
}

func crayCompilerName(flavor string) string {
	switch flavor {
	case "cce":
		return "cce"
	case "gcc":
		return "gcc"
	case "aocc":
		return "aocc"
	case "intel":
		return "intel"
	case "rocmcc":
		return "rocmcc"
	case "nvhpc":
		return "nvhpc"
	default:
		return ""
	}
}

func crayCompilerVersionFromEnv(flavor string, verification moduleVerification) string {
	if flavor == "cce" {
		return verification.Env["CRAY_CC_VERSION"]
	}
	return ""
}

func setCrayCompiler(vendor *model.VendorCray, flavor string, compiler *model.CrayCompilerBlock) {
	switch flavor {
	case "cce":
		vendor.CCE = compiler
	case "gcc":
		vendor.GCC = compiler
	case "aocc":
		vendor.AOCC = compiler
	case "intel":
		vendor.Intel = compiler
	case "rocmcc":
		vendor.ROCmCC = compiler
	case "nvhpc":
		vendor.NVHPC = compiler
	}
}

func getCrayCompiler(vendor *model.VendorCray, flavor string) *model.CrayCompilerBlock {
	switch flavor {
	case "cce":
		return vendor.CCE
	case "gcc":
		return vendor.GCC
	case "aocc":
		return vendor.AOCC
	case "intel":
		return vendor.Intel
	case "rocmcc":
		return vendor.ROCmCC
	case "nvhpc":
		return vendor.NVHPC
	default:
		return nil
	}
}

func verifiedCrayCompilerFlavors(vendor *model.VendorCray) []string {
	flavors := []string{}
	for _, flavor := range []string{"cce", "gcc", "aocc", "intel", "rocmcc", "nvhpc"} {
		compiler := getCrayCompiler(vendor, flavor)
		if compiler != nil && len(compiler.Modules) > 0 {
			flavors = append(flavors, flavor)
		}
	}
	return flavors
}

func filterModuleNames(modules []string, keep func(string) bool) []string {
	out := []string{}
	for _, module := range modules {
		if keep(module) {
			out = append(out, module)
		}
	}
	return out
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
