package probes

import (
	"os"
	"path/filepath"
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

func TestEnumerateModulePath(t *testing.T) {
	root := t.TempDir()
	writeTestModulefile(t, root, "gcc", "13.3.0.lua")
	writeTestModulefile(t, root, "openmpi", "5.0.9")
	writeTestModulefile(t, root, "rocm", "6.0.0.tcl")
	writeTestModulefile(t, root, ".hidden", "skip.lua")
	writeTestModulefile(t, root, "gcc", ".version")

	got := enumerateModulePath(root)
	want := []string{"gcc/13.3.0", "openmpi/5.0.9", "rocm/6.0.0"}
	assertStringSlicesEqual(t, got, want)
}

func TestEnumerateModuleCandidatesClassifiesAndSorts(t *testing.T) {
	root := t.TempDir()
	writeTestModulefile(t, root, "openmpi", "5.0.9.lua")
	writeTestModulefile(t, root, "gcc-native", "13.lua")

	got := enumerateModuleCandidates([]string{root})
	if len(got) != 2 {
		t.Fatalf("len(candidates) = %d, want 2: %#v", len(got), got)
	}
	if got[0].Name != "gcc-native/13" || !containsString(got[0].Categories, "compiler") {
		t.Fatalf("unexpected first candidate: %#v", got[0])
	}
	if got[1].Name != "openmpi/5.0.9" || !containsString(got[1].Categories, "mpi") {
		t.Fatalf("unexpected second candidate: %#v", got[1])
	}
}

func TestParseModuleAvail(t *testing.T) {
	out := `---------------- /opt/site/modules ----------------
gcc/13.3.0(D)   openmpi/5.0.9
rocm/6.0.0(L),  /absolute/path
MODULEPATH=/skip/me
`
	got := parseModuleAvail(out)
	want := []string{"gcc/13.3.0", "openmpi/5.0.9", "rocm/6.0.0"}
	assertStringSlicesEqual(t, got, want)
}

func TestClassifyModuleName(t *testing.T) {
	cases := []struct {
		name         string
		wantCategory string
	}{
		{"gcc-native/13", "compiler"},
		{"gnu/gcc/13", "compiler"},
		{"amd/aocc/4.2", "compiler"},
		{"openmpi/5.0.9", "mpi"},
		{"amd/openmpi/4.5.6", "mpi"},
		{"rocm/6.0.0", "gpu_toolkit"},
		{"amd/rocm/6.0.0", "gpu_toolkit"},
		{"nvidia/cuda/12.4", "gpu_toolkit"},
		{"libfabric/1.20", "fabric_userspace"},
		{"ofi/libfabric/1.20", "fabric_userspace"},
		{"cray-mpich/8.1.29", "mpi"},
		{"cray/PrgEnv-gnu", "compiler"},
		{"cray/cray-mpich/8.1.29", "cray_pe"},
		{"unclassified/1.0", unknownModuleCategory},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyModuleName(tc.name)
			if !containsString(got, tc.wantCategory) {
				t.Fatalf("classifyModuleName(%q) = %#v, want category %q", tc.name, got, tc.wantCategory)
			}
		})
	}
}

func TestAmbiguousModuleCandidates(t *testing.T) {
	got := ambiguousModuleCandidates([]ModuleCandidate{
		{Name: "cray-mpich/8.1.29", Categories: []string{"mpi", "cray_pe"}},
		{Name: "unclassified/1.0", Categories: []string{unknownModuleCategory}},
	})
	assertStringSlicesEqual(t, got, []string{"cray-mpich/8.1.29:mpi+cray_pe"})
}

func writeTestModulefile(t *testing.T, root string, parts ...string) {
	t.Helper()
	path := filepath.Join(append([]string{root}, parts...)...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("#%Module\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func assertStringSlicesEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
