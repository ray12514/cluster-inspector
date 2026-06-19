package probes

import (
	"os"
	"path/filepath"
	"strings"

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
//
// TODO: Phase 4 — add module candidate enumeration, hints filtering, and
// clean-shell verification.
func ProbeCompilersExternal() CompilersResult {
	result := CompilersResult{Evidence: map[string]model.Evidence{}}
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
		result.Compilers = append(result.Compilers, compiler)
		appendEvidence(result.Evidence, "compilers_external."+compiler.Name, evidence(model.ConfidenceProbed, compiler.Prefix))
	}
	if len(result.Compilers) == 0 {
		appendEvidence(result.Evidence, "compilers_external", evidence(model.ConfidenceUnknown, "no generic compiler commands found"))
	}
	return result
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
