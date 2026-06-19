package probes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseModuleVerification(t *testing.T) {
	got := parseModuleVerification("ENV:AOCC_HOME=/opt/aocc\nCMD:amdclang=/opt/aocc/bin/amdclang\n")
	if got.Env["AOCC_HOME"] != "/opt/aocc" {
		t.Fatalf("AOCC_HOME = %q", got.Env["AOCC_HOME"])
	}
	if got.Commands["amdclang"] != "/opt/aocc/bin/amdclang" {
		t.Fatalf("amdclang command = %q", got.Commands["amdclang"])
	}
}

func TestVerifyModulesWithFakeModulecmd(t *testing.T) {
	dir := t.TempDir()
	writeFakeModulecmd(t, dir)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	verification, err := verifyModules([]string{"aocc/4.2", "rocm/6.0.0"})
	if err != nil {
		t.Fatalf("verifyModules returned error: %v", err)
	}
	assertStringSlicesEqual(t, verification.Modules, []string{"aocc/4.2", "rocm/6.0.0"})
	if verification.Env["AOCC_HOME"] != "/opt/AMD/aocc-compiler-4.2" {
		t.Fatalf("AOCC_HOME = %q", verification.Env["AOCC_HOME"])
	}
	if verification.Env["ROCM_PATH"] != "/opt/rocm-6.0.0" {
		t.Fatalf("ROCM_PATH = %q", verification.Env["ROCM_PATH"])
	}
}

func TestVerificationBuilders(t *testing.T) {
	verification := moduleVerification{
		Modules: []string{"aocc/4.2"},
		Env: map[string]string{
			"AOCC_HOME": "/opt/AMD/aocc-compiler-4.2",
			"MPI_HOME":  "/opt/site/openmpi/5.0.9",
			"ROCM_PATH": "/opt/rocm-6.0.0",
		},
		Commands: map[string]string{},
	}

	compiler, ok := compilerExternalFromVerification("aocc", "aocc/4.2", verification)
	if !ok {
		t.Fatal("expected compiler external from verification")
	}
	if compiler.Name != "aocc" || compiler.Version != "4.2" || compiler.Prefix != "/opt/AMD/aocc-compiler-4.2" {
		t.Fatalf("unexpected compiler external: %#v", compiler)
	}

	mpi, ok := mpiExternalFromVerification("openmpi", "openmpi/5.0.9", verification)
	if !ok {
		t.Fatal("expected MPI external from verification")
	}
	if mpi.Name != "openmpi" || mpi.Version != "5.0.9" || mpi.Prefix != "/opt/site/openmpi/5.0.9" {
		t.Fatalf("unexpected MPI external: %#v", mpi)
	}

	rocm, ok := rocmToolkitFromVerification("rocm/6.0.0", verification)
	if !ok {
		t.Fatal("expected ROCm toolkit from verification")
	}
	if rocm.Version != "6.0.0" || rocm.Prefix != "/opt/rocm-6.0.0" {
		t.Fatalf("unexpected ROCm toolkit: %#v", rocm)
	}
	if len(rocm.SpackComponents) < 4 {
		t.Fatalf("expected ROCm component externals, got %#v", rocm.SpackComponents)
	}
}

func writeFakeModulecmd(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "modulecmd")
	script := `#!/bin/sh
if [ "$1" != "bash" ]; then exit 2; fi
if [ "$2" = "purge" ]; then exit 0; fi
if [ "$2" != "load" ]; then exit 2; fi
case "$3" in
  aocc/4.2)
    printf '%s\n' 'export AOCC_HOME=/opt/AMD/aocc-compiler-4.2'
    ;;
  rocm/6.0.0)
    printf '%s\n' 'export ROCM_PATH=/opt/rocm-6.0.0'
    ;;
  *)
    exit 1
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
