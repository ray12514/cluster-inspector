package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ray12514/cluster-inspector/internal/model"
	"github.com/ray12514/cluster-inspector/internal/output"
	"github.com/spf13/cobra"
)

func TestVerifyProfileFixturesPass(t *testing.T) {
	for _, name := range []string{"example-cray", "example-linux"} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			if err := verifyProfile(cmd, fixtureProfilePath(t, name)); err != nil {
				t.Fatalf("verifyProfile(%s) returned error: %v", name, err)
			}
			got := out.String()
			if !strings.Contains(got, "PASS schema") || !strings.Contains(got, "PASS semantic") {
				t.Fatalf("unexpected verify output: %q", got)
			}
		})
	}
}

func TestValidateProfileSemanticsRejectsMissingRuntime(t *testing.T) {
	profile := minimalSemanticProfile()
	profile.NodeTypes = map[string]model.NodeType{
		"login": profile.NodeTypes["login"],
	}
	errs := validateProfileSemantics(profile)
	assertSemanticErrorContains(t, errs, "runtime")
}

func TestValidateProfileSemanticsRejectsAMDWithoutROCm(t *testing.T) {
	profile := minimalSemanticProfile()
	node := profile.NodeTypes["runtime"]
	node.GPU = &model.GPUBlock{Vendor: "amd", DriverVersion: "6.0", ToolkitCeiling: "6.0.0", ArchTarget: "gfx90a"}
	profile.NodeTypes["runtime"] = node
	errs := validateProfileSemantics(profile)
	assertSemanticErrorContains(t, errs, "gpu_toolkit_modules.rocm")
}

func TestValidateProfileSemanticsRejectsUnknownCrayMPICHFlavor(t *testing.T) {
	profile := minimalSemanticProfile()
	profile.MPIProviders = []model.MPIProvider{{
		Name: "cray-mpich", Version: "8.1.29", ProviderFamily: "cray-pe",
		Flavors: map[string]model.MPIFlavor{
			"unknown": {Prefix: "/opt/cray/pe/mpich/8.1.29", Modules: []string{"cray-mpich/8.1.29"}},
		},
	}}
	errs := validateProfileSemantics(profile)
	assertSemanticErrorContains(t, errs, "unsupported flavor unknown")
}

func TestValidateProfileSemanticsRejectsNonEthernetWithoutDrivers(t *testing.T) {
	profile := minimalSemanticProfile()
	profile.Fabric = model.Fabric{Type: "slingshot", Drivers: []model.NamedPrefixVersioned{}}
	errs := validateProfileSemantics(profile)
	assertSemanticErrorContains(t, errs, "non-ethernet fabric must include at least one fabric driver")
}

func TestValidateProfileSemanticsRejectsMPICompilerNotInExternals(t *testing.T) {
	profile := minimalSemanticProfile()
	profile.CompilerProviders = []model.CompilerProvider{{Name: "gcc", Version: "13.2.0"}}
	profile.MPIProviders = []model.MPIProvider{{Name: "openmpi", Version: "5.0.0", Compiler: "aocc@4.2"}}
	errs := validateProfileSemantics(profile)
	assertSemanticErrorContains(t, errs, "does not match compiler_providers")
}

func TestVerifyProfileReportsSemanticFailure(t *testing.T) {
	profile := minimalSemanticProfile()
	profile.NodeTypes = map[string]model.NodeType{"login": profile.NodeTypes["login"]}
	path := writeProfileFixture(t, profile)

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := verifyProfile(cmd, path)
	if err == nil {
		t.Fatal("expected semantic verification error")
	}
	if !strings.Contains(out.String(), "PASS schema") || !strings.Contains(out.String(), "FAIL semantic") {
		t.Fatalf("unexpected verify output: %q", out.String())
	}
}

func fixtureProfilePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(fixtureDirPath(t, name), "profile.yaml")
}

func fixtureDirPath(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "tests", "fixtures", name)
}

func minimalSemanticProfile() *model.Profile {
	return &model.Profile{
		SchemaVersion: 1,
		System:        model.System{Name: "minimal", Family: "linux-rhel9"},
		OS:            model.OS{Name: "rhel", Major: 9, Glibc: "2.34"},
		Fabric:        model.Fabric{Type: "ethernet", Drivers: []model.NamedPrefixVersioned{}},
		ModulesSystem: model.ModulesSystem{Tool: "tcl"},
		Filesystem: model.Filesystem{InstallTreeCandidates: []model.InstallTreeCandidate{{
			Path:         "/opt/spack/opt",
			Type:         "overlay",
			LocksHonored: true,
		}}},
		NodeTypes: map[string]model.NodeType{
			"login": {
				Role: "build_host",
				CPU:  model.CPU{Detected: "x86_64", Preferred: "x86_64"},
				GPU:  nil,
				BuildStage: []model.BuildStage{{
					Path:       "/tmp/stage",
					Visibility: "node-local",
					Writable:   true,
				}},
			},
			"runtime": {
				Role: "runtime",
				CPU:  model.CPU{Detected: "x86_64", Preferred: "x86_64"},
				GPU:  nil,
				BuildStage: []model.BuildStage{{
					Path:       "/tmp/stage",
					Visibility: "node-local",
					Writable:   true,
				}},
			},
		},
	}
}

func writeProfileFixture(t *testing.T, profile *model.Profile) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "profile.yaml")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(%q): %v", path, err)
	}
	if err := output.WriteProfile(file, profile); err != nil {
		_ = file.Close()
		t.Fatalf("WriteProfile: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(%q): %v", path, err)
	}
	return path
}

func assertSemanticErrorContains(t *testing.T, errs []string, want string) {
	t.Helper()
	for _, err := range errs {
		if strings.Contains(err, want) {
			return
		}
	}
	t.Fatalf("semantic errors %#v do not contain %q", errs, want)
}
