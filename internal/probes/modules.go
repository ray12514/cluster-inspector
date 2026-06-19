package probes

import (
	"os"

	"github.com/ray12514/cluster-inspector/internal/model"
)

// ModulesResult contains the detected module system and MODULEPATH roots.
type ModulesResult struct {
	ModulesSystem model.ModulesSystem
	ModulePaths   []string
	Evidence      map[string]model.Evidence
}

// ProbeModules detects the module tool (Lmod vs Tcl) and enumerates
// MODULEPATH plus available modules.
//
// TODO: Phase 4 — full enumeration via `module avail -t`, MODULEPATH
// walks, classification, hints filtering, clean-shell verification.
func ProbeModules() ModulesResult {
	result := ModulesResult{
		ModulePaths: envList("MODULEPATH"),
		Evidence:    map[string]model.Evidence{},
	}

	lmodVersion := os.Getenv("LMOD_VERSION")
	modulesHome := firstNonEmptyEnv("MODULESHOME", "MODULEHOME")
	switch {
	case lmodVersion != "":
		result.ModulesSystem.Tool = "lmod"
		result.ModulesSystem.Version = lmodVersion
		appendEvidence(result.Evidence, "modules_system.tool", evidence(model.ConfidenceProbed, "LMOD_VERSION"))
		appendEvidence(result.Evidence, "modules_system.version", evidence(model.ConfidenceProbed, "LMOD_VERSION"))
	case modulesHome != "":
		result.ModulesSystem.Tool = "tcl"
		appendEvidence(result.Evidence, "modules_system.tool", evidence(model.ConfidenceProbed, "MODULESHOME/MODULEHOME"))
	default:
		result.ModulesSystem.Tool = detectModuleToolFromPath(result.Evidence)
	}

	if result.ModulesSystem.Version == "" {
		result.ModulesSystem.Version = detectModuleVersion(result.ModulesSystem.Tool, result.Evidence)
	}
	if len(result.ModulePaths) > 0 {
		appendEvidence(result.Evidence, "module_paths", evidence(model.ConfidenceProbed, "MODULEPATH"))
	} else {
		appendEvidence(result.Evidence, "module_paths", evidence(model.ConfidenceUnknown, "MODULEPATH unset"))
	}

	return result
}

func detectModuleToolFromPath(evidenceMap map[string]model.Evidence) string {
	if commandPath("lmod") != "" || commandPath("ml") != "" {
		appendEvidence(evidenceMap, "modules_system.tool", evidence(model.ConfidenceInferred, "lmod/ml command on PATH"))
		return "lmod"
	}
	if commandPath("modulecmd") != "" {
		appendEvidence(evidenceMap, "modules_system.tool", evidence(model.ConfidenceInferred, "modulecmd on PATH"))
		return "tcl"
	}
	appendEvidence(evidenceMap, "modules_system.tool", evidence(model.ConfidenceUnknown, "LMOD_VERSION/MODULESHOME/MODULEHOME/modulecmd unavailable"))
	return ""
}

func detectModuleVersion(tool string, evidenceMap map[string]model.Evidence) string {
	if tool == "tcl" && commandPath("modulecmd") != "" {
		if out, err := run("modulecmd", "--version"); err == nil {
			if version := firstVersion(out); version != "" {
				appendEvidence(evidenceMap, "modules_system.version", evidence(model.ConfidenceProbed, "modulecmd --version"))
				return version
			}
		}
	}
	if tool == "lmod" {
		if out, err := run("lmod", "--version"); err == nil {
			if version := firstVersion(out); version != "" {
				appendEvidence(evidenceMap, "modules_system.version", evidence(model.ConfidenceProbed, "lmod --version"))
				return version
			}
		}
	}
	appendEvidence(evidenceMap, "modules_system.version", evidence(model.ConfidenceUnknown, "module version unavailable"))
	return ""
}
