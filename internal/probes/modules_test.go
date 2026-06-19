package probes

import (
	"testing"

	"github.com/ray12514/cluster-inspector/internal/model"
)

func TestModulesEnvSignalsLmod(t *testing.T) {
	t.Setenv("LMOD_VERSION", "8.7.32")
	t.Setenv("MODULESHOME", "")
	t.Setenv("MODULEHOME", "")
	t.Setenv("MODULEPATH", "")

	r := ProbeModules()
	if r.ModulesSystem.Tool != "lmod" {
		t.Fatalf("tool: got %q, want lmod", r.ModulesSystem.Tool)
	}
	if r.ModulesSystem.Version != "8.7.32" {
		t.Fatalf("version: got %q, want 8.7.32", r.ModulesSystem.Version)
	}
	if r.Evidence["modules_system.tool"].Confidence != model.ConfidenceProbed {
		t.Fatalf("expected probed confidence for tool")
	}
}

func TestModulesEnvSignalsTcl(t *testing.T) {
	t.Setenv("LMOD_VERSION", "")
	t.Setenv("MODULESHOME", "/usr/share/Modules")
	t.Setenv("MODULEPATH", "")

	r := ProbeModules()
	if r.ModulesSystem.Tool != "tcl" {
		t.Fatalf("tool: got %q, want tcl", r.ModulesSystem.Tool)
	}
	if r.Evidence["modules_system.tool"].Confidence != model.ConfidenceProbed {
		t.Fatalf("expected probed confidence for tool")
	}
}

func TestModulesNoEvidenceLeavesUnknown(t *testing.T) {
	t.Setenv("LMOD_VERSION", "")
	t.Setenv("MODULESHOME", "")
	t.Setenv("MODULEHOME", "")
	t.Setenv("MODULEPATH", "")

	r := ProbeModules()
	// On a host with no module tool, modulecmd/lmod/ml binaries on PATH
	// could still set the tool. Accept any of: "" (truly nothing), or a
	// detected tool with inferred confidence.
	tool := r.ModulesSystem.Tool
	if tool != "" && tool != "lmod" && tool != "tcl" {
		t.Fatalf("unexpected tool %q", tool)
	}
	if tool == "" {
		if r.Evidence["modules_system.tool"].Confidence != model.ConfidenceUnknown {
			t.Fatalf("expected unknown confidence when no evidence found, got %q",
				r.Evidence["modules_system.tool"].Confidence)
		}
	}
	if len(r.ModulePaths) != 0 {
		t.Fatalf("ModulePaths should be empty when MODULEPATH unset, got %v", r.ModulePaths)
	}
	if r.Evidence["module_paths"].Confidence != model.ConfidenceUnknown {
		t.Fatalf("expected unknown confidence for module_paths")
	}
}

func TestModulesPathParsing(t *testing.T) {
	t.Setenv("LMOD_VERSION", "")
	t.Setenv("MODULESHOME", "")
	t.Setenv("MODULEHOME", "")
	t.Setenv("MODULEPATH", "/opt/cray/pe/lmod/modulefiles/core:/opt/site/modules:")

	r := ProbeModules()
	want := []string{"/opt/cray/pe/lmod/modulefiles/core", "/opt/site/modules"}
	if len(r.ModulePaths) != len(want) {
		t.Fatalf("ModulePaths len: got %d (%v), want %d (%v)",
			len(r.ModulePaths), r.ModulePaths, len(want), want)
	}
	for i, p := range want {
		if r.ModulePaths[i] != p {
			t.Fatalf("ModulePaths[%d]: got %q, want %q", i, r.ModulePaths[i], p)
		}
	}
}
