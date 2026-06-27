package probes

import (
	"os"
	"path/filepath"
	"sort"

	inspectorhints "github.com/ray12514/cluster-inspector/internal/hints"
	"github.com/ray12514/cluster-inspector/internal/model"
)

// CrayPEResult contains generic provider facts discovered from Cray PE/CPE
// evidence when present. Cray-specific probing must not leak a Cray-shaped
// fragment contract.
type CrayPEResult struct {
	CompilerProviders []model.CompilerProvider
	MPIProviders      []model.MPIProvider
	Evidence          map[string]model.Evidence
}

type crayPEInventory struct {
	PEVersion string
	Compilers map[string]*crayCompilerBlock
	CrayMPICH *crayMPICHBlock
}

type crayCompilerBlock struct {
	Version string
	Prefix  string
	Modules []string
}

type crayMPICHBlock struct {
	Version string
	Flavors map[string]crayMPICHFlavor
}

type crayMPICHFlavor struct {
	Prefix  string
	Modules []string
}

// ProbeCrayPE detects Cray Programming Environment presence and inventory:
// PE version, platform-owned compilers, and Cray MPICH flavors
// (per-compiler prefixes).
func ProbeCrayPE() CrayPEResult {
	return ProbeCrayPEWithModules(nil, nil)
}

// ProbeCrayPEWithModules detects Cray PE inventory from filesystem/env
// evidence and verified module combinations.
func ProbeCrayPEWithModules(candidates []ModuleCandidate, hints *inspectorhints.Hints) CrayPEResult {
	result := CrayPEResult{Evidence: map[string]model.Evidence{}}
	if !detectCrayEvidence() && !crayModuleEvidencePresent(candidates) {
		appendEvidence(result.Evidence, "provider.platform.cray-pe", evidence(model.ConfidenceProbed, "no Cray PE evidence"))
		return result
	}

	peVersion := os.Getenv("CRAYPE_VERSION")
	if peVersion == "" {
		peVersion = latestChildVersion(crayPEPolicy().PERoot)
	}
	if peVersion == "" {
		peVersion = "unknown"
		appendEvidence(result.Evidence, "provider.platform.cray-pe.version", evidence(model.ConfidenceUnknown, "Cray PE present; version unavailable"))
	} else {
		appendEvidence(result.Evidence, "provider.platform.cray-pe.version", evidence(model.ConfidenceProbed, "CRAYPE_VERSION or "+crayPEPolicy().PERoot))
	}

	vendor := &crayPEInventory{
		PEVersion: peVersion,
		Compilers: map[string]*crayCompilerBlock{},
	}
	peRoot := crayPEPolicy().PERoot
	setCrayCompiler(vendor, "cce", crayCompiler("cce", firstNonEmptyEnv(compilerEnvKeys("cce")...), filepath.Join(peRoot, "cce"), []string{compilerProgramEnvironmentModule("cce")}))
	setCrayCompiler(vendor, "gcc", crayCompiler("gcc", firstNonEmptyEnv(compilerEnvKeys("gcc")...), filepath.Join(peRoot, "gcc"), []string{compilerProgramEnvironmentModule("gcc")}))
	setCrayCompiler(vendor, "aocc", crayAOCCCompiler())
	setCrayCompiler(vendor, "intel", crayIntelCompiler())
	rocmPolicy, _ := gpuToolkitPolicyByName("rocm")
	nvhpcPolicy, _ := gpuToolkitPolicyByName("nvhpc")
	setCrayCompiler(vendor, "rocmcc", crayCompiler("rocmcc", firstNonEmptyEnv(compilerEnvKeys("rocmcc")...), firstExistingPolicyRoot(rocmPolicy.Roots), []string{compilerProgramEnvironmentModule("rocmcc")}))
	setCrayCompiler(vendor, "nvhpc", crayCompiler("nvhpc", firstNonEmptyEnv(compilerEnvKeys("nvhpc")...), firstExistingPolicyRoot(nvhpcPolicy.Roots), []string{compilerProgramEnvironmentModule("nvhpc")}))
	vendor.CrayMPICH = crayMPICH()
	applyVerifiedCrayModules(vendor, candidates, hints, result.Evidence)

	for _, name := range sortedCrayCompilerNames(vendor) {
		appendCrayCompilerEvidence(result.Evidence, name, getCrayCompiler(vendor, name))
	}
	if vendor.CrayMPICH != nil {
		appendEvidence(result.Evidence, "mpi_providers.cray-mpich", evidence(model.ConfidenceProbed, "Cray MPICH environment or "+filepath.Join(crayPEPolicy().PERoot, "mpich")))
	}
	result.CompilerProviders = crayCompilerProviders(vendor)
	result.MPIProviders = crayMPIProviders(vendor)
	return result
}

func crayCompilerProviders(vendor *crayPEInventory) []model.CompilerProvider {
	if vendor == nil {
		return nil
	}
	langs := []string{"c", "c++", "fortran"}
	out := []model.CompilerProvider{}
	for _, name := range sortedCrayCompilerNames(vendor) {
		block := getCrayCompiler(vendor, name)
		if block == nil {
			continue
		}
		out = append(out, model.CompilerProvider{
			Name:           name,
			Version:        block.Version,
			Prefix:         block.Prefix,
			ProviderFamily: "platform",
			PlatformFamily: "cray-pe",
			Languages:      langs,
			Modules:        block.Modules,
		})
	}
	return out
}

func crayMPIProviders(vendor *crayPEInventory) []model.MPIProvider {
	if vendor == nil || vendor.CrayMPICH == nil {
		return nil
	}
	flavors := map[string]model.MPIFlavor{}
	compilers := make([]string, 0, len(vendor.CrayMPICH.Flavors))
	for name, flavor := range vendor.CrayMPICH.Flavors {
		flavors[name] = model.MPIFlavor{Prefix: flavor.Prefix, Modules: flavor.Modules}
		compilers = append(compilers, name)
	}
	sort.Strings(compilers)
	return []model.MPIProvider{{
		Name:           "cray-mpich",
		Version:        vendor.CrayMPICH.Version,
		ProviderFamily: "platform",
		PlatformFamily: "cray-pe",
		Compatibility:  &model.MPICompatibility{Compilers: compilers},
		Flavors:        flavors,
	}}
}

func crayModuleEvidencePresent(candidates []ModuleCandidate) bool {
	for _, candidate := range candidates {
		if stringSliceContains(candidate.Categories, "cray_pe") {
			return true
		}
		if moduleHasSegmentPrefix(candidate.Name, crayPEPolicy().ModulePrefixes...) ||
			moduleHasSegment(candidate.Name, crayPEPolicy().ModuleSegments...) {
			return true
		}
		if mpiPolicyPlatformOwned(mpiNameFromModule(candidate.Name)) {
			return true
		}
	}
	return false
}

func applyVerifiedCrayModules(vendor *crayPEInventory, candidates []ModuleCandidate, hints *inspectorhints.Hints, evidenceMap map[string]model.Evidence) {
	if vendor == nil {
		return
	}
	moduleHints := inspectorhints.ModuleHints{}
	if hints != nil {
		moduleHints = hints.Compilers
	}
	compilerCandidates := append(candidateNamesByCategory(candidates, "compiler"), candidateNamesByCategory(candidates, "cray_pe")...)
	acceptedCompilers := applyModulePolicy(compilerCandidates, moduleHints, nil, evidenceMap, "compiler_providers.platform.cray-pe.module_hints")
	for _, module := range acceptedCompilers {
		flavor := crayFlavorFromModule(module)
		if flavor == "" {
			continue
		}
		moduleSet := crayCompilerModuleSet(flavor, module)
		verification, err := verifyModules(moduleSet)
		if err != nil {
			appendVerificationFailure(evidenceMap, "compiler_providers.platform.cray-pe.verify_failed."+module, moduleSet, err)
			continue
		}
		compiler, ok := crayCompilerFromVerification(flavor, module, verification)
		if !ok {
			appendEvidence(evidenceMap, "compiler_providers.platform.cray-pe.verify_failed."+module, evidence(model.ConfidenceUnknown, "module loaded but compiler prefix unavailable"))
			continue
		}
		setCrayCompiler(vendor, flavor, compiler)
		appendEvidence(evidenceMap, "compiler_providers.platform.cray-pe."+flavor, evidence(model.ConfidenceProbed, "clean-shell module verification"))
	}
	applyVerifiedCrayMPICH(vendor, candidates, hints, evidenceMap)
}

func applyVerifiedCrayMPICH(vendor *crayPEInventory, candidates []ModuleCandidate, hints *inspectorhints.Hints, evidenceMap map[string]model.Evidence) {
	moduleHints := inspectorhints.ModuleHints{}
	if hints != nil {
		moduleHints = hints.MPI
	}
	mpichCandidates := filterModuleNames(candidateNamesByCategory(candidates, "mpi"), func(module string) bool {
		return mpiNameFromModule(module) == "cray-mpich"
	})
	accepted := applyModulePolicy(mpichCandidates, moduleHints, nil, evidenceMap, "mpi_providers.platform.cray-pe.cray-mpich.module_hints")
	for _, mpichModule := range accepted {
		version := moduleVersion(mpichModule)
		if version == "" {
			version = "unknown"
		}
		if vendor.CrayMPICH == nil {
			vendor.CrayMPICH = &crayMPICHBlock{Version: version, Flavors: map[string]crayMPICHFlavor{}}
		}
		if vendor.CrayMPICH.Flavors == nil {
			vendor.CrayMPICH.Flavors = map[string]crayMPICHFlavor{}
		}
		for _, flavor := range verifiedCrayCompilerFlavors(vendor) {
			compiler := getCrayCompiler(vendor, flavor)
			moduleSet := append([]string{}, compiler.Modules...)
			moduleSet = append(moduleSet, mpichModule)
			verification, err := verifyModules(moduleSet)
			if err != nil {
				appendVerificationFailure(evidenceMap, "mpi_providers.platform.cray-pe.cray-mpich.verify_failed."+flavor, moduleSet, err)
				continue
			}
			prefix := firstNonEmptyString(verification.Env["MPICH_DIR"], prefixFromCommand(verification.Commands["mpicc"]))
			if prefix == "" {
				appendEvidence(evidenceMap, "mpi_providers.platform.cray-pe.cray-mpich.verify_failed."+flavor, evidence(model.ConfidenceUnknown, "module loaded but MPICH prefix unavailable"))
				continue
			}
			vendor.CrayMPICH.Version = firstNonEmptyString(firstNonEmptyString(mpiVersionEnvValues("cray-mpich", verification)...), version)
			vendor.CrayMPICH.Flavors[flavor] = crayMPICHFlavor{Prefix: prefix, Modules: []string{mpichModule}}
			appendEvidence(evidenceMap, "mpi_providers.platform.cray-pe.cray-mpich."+flavor, evidence(model.ConfidenceProbed, "clean-shell module verification"))
		}
	}
}

func crayFlavorFromModule(module string) string {
	return compilerNameFromModule(module)
}

func crayCompilerModuleSet(flavor, module string) []string {
	moduleSet := []string{}
	if prgEnv := compilerProgramEnvironmentModule(flavor); prgEnv != "" {
		moduleSet = append(moduleSet, prgEnv)
	}
	if !moduleHasSegmentPrefix(module, crayPEPolicy().ModulePrefixes...) {
		moduleSet = append(moduleSet, module)
	}
	return cleanModuleList(moduleSet)
}

func crayCompilerFromVerification(flavor, module string, verification moduleVerification) (*crayCompilerBlock, bool) {
	name := flavor
	prefix := compilerPrefixFromVerification(name, verification)
	if prefix == "" && flavor == "cce" {
		prefix = verification.Env["CRAY_PE_CCE_PREFIX"]
	}
	if prefix == "" {
		return nil, false
	}
	version := firstNonEmptyString(firstNonEmptyString(compilerVersionEnvValues(name, verification)...), moduleVersion(module), firstVersion(prefix))
	if version == "" {
		version = "unknown"
	}
	return &crayCompilerBlock{Version: version, Prefix: prefix, Modules: verification.Modules}, true
}

func setCrayCompiler(vendor *crayPEInventory, flavor string, compiler *crayCompilerBlock) {
	if vendor == nil || compiler == nil || flavor == "" {
		return
	}
	if vendor.Compilers == nil {
		vendor.Compilers = map[string]*crayCompilerBlock{}
	}
	vendor.Compilers[flavor] = compiler
}

func getCrayCompiler(vendor *crayPEInventory, flavor string) *crayCompilerBlock {
	if vendor == nil {
		return nil
	}
	return vendor.Compilers[flavor]
}

func sortedCrayCompilerNames(vendor *crayPEInventory) []string {
	if vendor == nil {
		return nil
	}
	names := make([]string, 0, len(vendor.Compilers))
	for name, compiler := range vendor.Compilers {
		if compiler != nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func verifiedCrayCompilerFlavors(vendor *crayPEInventory) []string {
	flavors := []string{}
	for _, flavor := range sortedCrayCompilerNames(vendor) {
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

func crayAOCCCompiler() *crayCompilerBlock {
	roots := append([]string{filepath.Join(crayPEPolicy().PERoot, "aocc")}, compilerPolicyRoots("aocc")...)
	return crayCompiler(
		"aocc",
		firstNonEmptyEnv(compilerEnvKeys("aocc")...),
		firstExistingDir(roots...),
		[]string{compilerProgramEnvironmentModule("aocc")},
	)
}

func crayIntelCompiler() *crayCompilerBlock {
	roots := append([]string{filepath.Join(crayPEPolicy().PERoot, "intel")}, compilerPolicyRoots("intel")...)
	return crayCompiler(
		"intel",
		firstNonEmptyEnv(compilerEnvKeys("intel")...),
		firstExistingDir(roots...),
		[]string{compilerProgramEnvironmentModule("intel")},
	)
}

func compilerProgramEnvironmentModule(name string) string {
	return firstNonEmptyString(compilerPolicyModulePrefixes(name)...)
}

func appendCrayCompilerEvidence(evidenceMap map[string]model.Evidence, name string, compiler *crayCompilerBlock) {
	if compiler == nil {
		return
	}
	appendEvidence(evidenceMap, "compiler_providers.platform.cray-pe."+name, evidence(model.ConfidenceProbed, compiler.Prefix))
}

func crayCompiler(name, envPrefix, fallbackRoot string, modules []string) *crayCompilerBlock {
	prefix := envPrefix
	if prefix == "" && isDir(fallbackRoot) {
		version := latestChildVersion(fallbackRoot)
		if version != "" {
			prefix = filepath.Join(fallbackRoot, version)
		} else {
			prefix = fallbackRoot
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
	return &crayCompilerBlock{
		Version: version,
		Prefix:  prefix,
		Modules: modules,
	}
}

func crayMPICH() *crayMPICHBlock {
	version := firstNonEmptyEnv(mpiVersionEnvKeys("cray-mpich")...)
	prefix := firstNonEmptyEnv(mpiEnvKeys("cray-mpich")...)
	if version == "" && prefix != "" {
		version = firstVersion(prefix)
	}
	mpichRoot := filepath.Join(crayPEPolicy().PERoot, "mpich")
	if prefix == "" && isDir(mpichRoot) {
		version = latestChildVersion(mpichRoot)
		if version != "" {
			prefix = filepath.Join(mpichRoot, version)
		}
	}
	if version == "" || prefix == "" {
		return nil
	}
	return &crayMPICHBlock{
		Version: version,
		Flavors: map[string]crayMPICHFlavor{
			crayCompilerFlavor(): {
				Prefix:  prefix,
				Modules: []string{"cray-mpich/" + version},
			},
		},
	}
}

func crayCompilerFlavor() string {
	if name := platformProviderFromEnv(crayPEPolicy(), "PE_ENV", os.Getenv("PE_ENV")); name != "" {
		return name
	}
	return "unknown"
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}
