package probes

import (
	"os"
	"path/filepath"
	"strings"

	inspectorhints "github.com/ray12514/cluster-inspector/internal/hints"
	"github.com/ray12514/cluster-inspector/internal/model"
)

// CompilersResult contains generic non-Cray compiler externals.
type CompilersResult struct {
	Compilers []model.CompilerExternal
	Evidence  map[string]model.Evidence
}

// ProbeCompilersExternal discovers generic site or system compilers
// (gcc, aocc, intel, oneapi, nvhpc, etc.) — excludes Cray PE compilers,
// which are handled by ProbeCrayPE.
func ProbeCompilersExternal() CompilersResult {
	return ProbeCompilersExternalWithModules(nil, nil)
}

// ProbeCompilersExternalWithModules discovers generic compilers from both
// the current environment and verified module candidates. Cray PE compiler
// modules are intentionally left for ProbeCrayPEWithModules.
func ProbeCompilersExternalWithModules(candidates []ModuleCandidate, hints *inspectorhints.Hints) CompilersResult {
	result := CompilersResult{Evidence: map[string]model.Evidence{}}
	seen := map[string]bool{}
	for _, candidate := range []compilerCandidate{
		{name: "gcc", cc: []string{"gcc"}, cxx: []string{"g++"}, fc: []string{"gfortran"}},
		{name: "aocc", cc: []string{"amdclang", "clang"}, cxx: []string{"amdclang++", "clang++"}, fc: []string{"flang"}, env: []string{"AOCC_HOME", "AOCC_ROOT", "AOMP"}},
		{name: "oneapi", cc: []string{"icx"}, cxx: []string{"icpx"}, fc: []string{"ifx"}, env: []string{"ONEAPI_ROOT", "CMPLR_ROOT"}},
		{name: "intel", cc: []string{"icc"}, cxx: []string{"icpc"}, fc: []string{"ifort"}, env: []string{"INTEL_PATH", "INTEL_HOME"}},
		{name: "nvhpc", cc: []string{"nvc"}, cxx: []string{"nvc++"}, fc: []string{"nvfortran"}, env: []string{"NVHPC_ROOT"}},
		{name: "clang", cc: []string{"clang"}, cxx: []string{"clang++"}},
	} {
		compiler, ok := probeCompiler(candidate)
		if !ok {
			continue
		}
		if activeCrayPrgEnvCompiler(compiler.Name) {
			continue
		}
		if compiler.Name == "clang" && compilerLooksLikeAOCC(compiler) {
			continue
		}
		if strings.HasPrefix(compiler.Prefix, "/opt/cray") {
			continue
		}
		appendCompilerExternal(&result.Compilers, seen, compiler)
		appendEvidence(result.Evidence, "compilers_external."+compiler.Name, evidence(model.ConfidenceProbed, compiler.Prefix))
	}
	for _, compiler := range verifiedCompilerModules(candidates, hints, result.Evidence) {
		appendCompilerExternal(&result.Compilers, seen, compiler)
	}
	for _, compiler := range compilerExtras(hints) {
		appendCompilerExternal(&result.Compilers, seen, compiler)
		appendEvidence(result.Evidence, "compilers_external.extra."+compiler.Name, evidence(model.ConfidenceInferred, "inspector-hints extras"))
	}
	if len(result.Compilers) == 0 {
		appendEvidence(result.Evidence, "compilers_external", evidence(model.ConfidenceUnknown, "no generic compiler commands found"))
	}
	return result
}

func verifiedCompilerModules(candidates []ModuleCandidate, hints *inspectorhints.Hints, evidenceMap map[string]model.Evidence) []model.CompilerExternal {
	moduleHints := inspectorhints.ModuleHints{}
	if hints != nil {
		moduleHints = hints.Compilers
	}
	accepted := applyModulePolicy(candidateNamesByCategory(candidates, "compiler"), moduleHints, nil, evidenceMap, "compilers_external.module_hints")
	compilers := []model.CompilerExternal{}
	for _, module := range accepted {
		name := compilerNameFromModule(module)
		if name == "" || isCrayCompilerModule(module) {
			continue
		}
		verification, err := verifyModules([]string{module})
		if err != nil {
			appendVerificationFailure(evidenceMap, "compilers_external.verify_failed."+module, []string{module}, err)
			continue
		}
		compiler, ok := compilerExternalFromVerification(name, module, verification)
		if !ok {
			appendEvidence(evidenceMap, "compilers_external.verify_failed."+module, evidence(model.ConfidenceUnknown, "module loaded but compiler prefix unavailable"))
			continue
		}
		compilers = append(compilers, compiler)
		appendEvidence(evidenceMap, "compilers_external.module."+module, evidence(model.ConfidenceProbed, "clean-shell module verification"))
	}
	return compilers
}

func compilerExternalFromVerification(name, module string, verification moduleVerification) (model.CompilerExternal, bool) {
	prefix := compilerPrefixFromVerification(name, verification)
	if prefix == "" || strings.HasPrefix(prefix, "/opt/cray") {
		return model.CompilerExternal{}, false
	}
	version := moduleVersion(module)
	if version == "" {
		version = firstVersion(prefix)
	}
	if version == "" {
		version = "unknown"
	}
	languages := compilerLanguagesFromVerification(name, verification, prefix)
	if len(languages) == 0 {
		languages = []string{"c"}
	}
	return model.CompilerExternal{
		Name:      name,
		Version:   version,
		Prefix:    prefix,
		Modules:   verification.Modules,
		Languages: languages,
	}, true
}

func compilerPrefixFromVerification(name string, verification moduleVerification) string {
	for _, key := range compilerEnvKeys(name) {
		if prefix := verification.Env[key]; prefix != "" {
			return prefix
		}
	}
	for _, command := range compilerCommands(name) {
		if commandPath := verification.Commands[command]; commandPath != "" {
			return prefixFromCommand(commandPath)
		}
	}
	return ""
}

func compilerLanguagesFromVerification(name string, verification moduleVerification, prefix string) []string {
	languages := []string{}
	for _, command := range compilerCommands(name) {
		commandPath := verification.Commands[command]
		if commandPath == "" || !samePrefix(prefix, commandPath) {
			continue
		}
		switch command {
		case "gcc", "clang", "amdclang", "icx", "icc", "nvc", "cc":
			if !stringSliceContains(languages, "c") {
				languages = append(languages, "c")
			}
		case "g++", "clang++", "amdclang++", "icpx", "icpc", "nvc++", "CC":
			if !stringSliceContains(languages, "c++") {
				languages = append(languages, "c++")
			}
		case "gfortran", "flang", "ifx", "ifort", "nvfortran", "ftn":
			if !stringSliceContains(languages, "fortran") {
				languages = append(languages, "fortran")
			}
		}
	}
	return languages
}

func compilerNameFromModule(module string) string {
	lower := strings.ToLower(module)
	switch {
	case strings.HasPrefix(lower, "prgenv-cray") || strings.HasPrefix(lower, "cce/"):
		return "cce"
	case strings.HasPrefix(lower, "prgenv-gnu") || strings.HasPrefix(lower, "gcc/") || strings.HasPrefix(lower, "gcc-native/"):
		return "gcc"
	case strings.HasPrefix(lower, "prgenv-aocc") || strings.HasPrefix(lower, "aocc/"):
		return "aocc"
	case strings.HasPrefix(lower, "prgenv-intel") || strings.HasPrefix(lower, "intel/"):
		return "intel"
	case strings.HasPrefix(lower, "oneapi/"):
		return "oneapi"
	case strings.HasPrefix(lower, "prgenv-nvidia") || strings.HasPrefix(lower, "nvhpc/"):
		return "nvhpc"
	case strings.HasPrefix(lower, "prgenv-amd") || strings.HasPrefix(lower, "rocmcc/"):
		return "rocmcc"
	default:
		return ""
	}
}

func isCrayCompilerModule(module string) bool {
	lower := strings.ToLower(module)
	return strings.HasPrefix(lower, "prgenv-") || strings.HasPrefix(lower, "cce/") || strings.HasPrefix(lower, "gcc-native/") || strings.HasPrefix(lower, "rocmcc/")
}

func compilerEnvKeys(name string) []string {
	switch name {
	case "gcc":
		return []string{"GCC_PATH"}
	case "aocc":
		return []string{"AOCC_HOME", "AOCC_ROOT", "AOMP"}
	case "intel", "oneapi":
		return []string{"ONEAPI_ROOT", "CMPLR_ROOT", "INTEL_PATH", "INTEL_HOME"}
	case "nvhpc":
		return []string{"NVHPC_ROOT"}
	case "rocmcc":
		return []string{"ROCM_PATH"}
	default:
		return nil
	}
}

func compilerCommands(name string) []string {
	switch name {
	case "gcc":
		return []string{"gcc", "g++", "gfortran"}
	case "aocc", "rocmcc":
		return []string{"amdclang", "amdclang++", "flang", "clang", "clang++"}
	case "intel", "oneapi":
		return []string{"icx", "icpx", "ifx", "icc", "icpc", "ifort"}
	case "nvhpc":
		return []string{"nvc", "nvc++", "nvfortran"}
	case "cce":
		return []string{"cc", "CC", "ftn"}
	default:
		return nil
	}
}

func compilerExtras(hints *inspectorhints.Hints) []model.CompilerExternal {
	if hints == nil {
		return nil
	}
	out := make([]model.CompilerExternal, 0, len(hints.Extras.Compilers))
	for _, extra := range hints.Extras.Compilers {
		out = append(out, model.CompilerExternal{
			Name:      extra.Name,
			Version:   extra.Version,
			Prefix:    extra.Prefix,
			Modules:   []string{extra.Module},
			Languages: extra.Languages,
		})
	}
	return out
}

func appendCompilerExternal(compilers *[]model.CompilerExternal, seen map[string]bool, compiler model.CompilerExternal) {
	key := compiler.Name + "@" + compiler.Version + ":" + compiler.Prefix
	if seen[key] {
		return
	}
	seen[key] = true
	*compilers = append(*compilers, compiler)
}

func activeCrayPrgEnvCompiler(name string) bool {
	switch os.Getenv("PE_ENV") {
	case "GNU":
		return name == "gcc"
	case "AOCC":
		return name == "aocc"
	case "INTEL":
		return name == "intel" || name == "oneapi"
	case "NVIDIA":
		return name == "nvhpc"
	default:
		return false
	}
}

type compilerCandidate struct {
	name string
	cc   []string
	cxx  []string
	fc   []string
	env  []string
}

func probeCompiler(candidate compilerCandidate) (model.CompilerExternal, bool) {
	cc, ccPath := firstCommand(candidate.cc...)
	if ccPath == "" {
		return model.CompilerExternal{}, false
	}
	version := compilerVersion(candidate.name, cc)
	if version == "" {
		return model.CompilerExternal{}, false
	}
	prefix := compilerPrefix(candidate, ccPath)
	if prefix == "" {
		return model.CompilerExternal{}, false
	}
	if candidate.name == "aocc" && cc != "amdclang" && !compilerLooksLikeAOCCPrefix(prefix) {
		return model.CompilerExternal{}, false
	}

	languages := []string{"c"}
	if _, cxxPath := firstCommand(candidate.cxx...); cxxPath != "" && samePrefix(prefix, cxxPath) {
		languages = append(languages, "c++")
	}
	if _, fcPath := firstCommand(candidate.fc...); fcPath != "" && samePrefix(prefix, fcPath) {
		languages = append(languages, "fortran")
	}
	return model.CompilerExternal{
		Name:      candidate.name,
		Version:   version,
		Prefix:    prefix,
		Languages: languages,
	}, true
}

func compilerVersion(name, command string) string {
	if name == "gcc" {
		if out, err := run(command, "-dumpfullversion", "-dumpversion"); err == nil {
			return firstVersion(out)
		}
	}
	if out, err := run(command, "--version"); err == nil {
		return firstVersion(out)
	}
	return ""
}

func compilerPrefix(candidate compilerCandidate, ccPath string) string {
	for _, name := range candidate.env {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return prefixFromCommand(ccPath)
}

func firstCommand(names ...string) (string, string) {
	for _, name := range names {
		if path := commandPath(name); path != "" {
			return name, path
		}
	}
	return "", ""
}

func compilerLooksLikeAOCC(compiler model.CompilerExternal) bool {
	return compilerLooksLikeAOCCPrefix(compiler.Prefix)
}

func compilerLooksLikeAOCCPrefix(prefix string) bool {
	prefix = strings.ToLower(prefix)
	return strings.Contains(prefix, "aocc") || strings.Contains(prefix, "/amd/")
}

func samePrefix(prefix, command string) bool {
	if command == "" {
		return false
	}
	cleanPrefix := filepath.Clean(prefix)
	cleanCommand := filepath.Clean(command)
	return filepath.Clean(prefixFromCommand(command)) == cleanPrefix || strings.HasPrefix(cleanCommand, cleanPrefix+string(filepath.Separator))
}
