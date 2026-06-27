package probes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ray12514/cluster-inspector/internal/model"
)

func TestDiscoveryPolicyLoadsCoreProviderRules(t *testing.T) {
	p := policy()
	if p.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", p.SchemaVersion)
	}
	if _, ok := compilerPolicyByName("gcc"); !ok {
		t.Fatal("expected gcc compiler policy")
	}
	rocm, ok := gpuToolkitPolicyByName("rocm")
	if !ok {
		t.Fatal("expected rocm toolkit policy")
	}
	if len(rocm.Roots) == 0 || rocm.Roots[0] != "/opt/rocm" {
		t.Fatalf("unexpected ROCm roots: %#v", rocm.Roots)
	}
	if len(rocm.ComponentCandidates) == 0 {
		t.Fatal("expected ROCm component policy")
	}
	if prefixes := compilerPolicyModulePrefixes("gcc"); len(prefixes) == 0 || prefixes[0] != "PrgEnv-gnu" {
		t.Fatalf("expected compiler module-name prefixes from policy, got %#v", prefixes)
	}
	if len(p.Filesystem.SharedProbeRoots) == 0 {
		t.Fatal("expected shared filesystem probe roots")
	}
	if len(p.Filesystem.ScratchProbeRoots) == 0 {
		t.Fatal("expected scratch filesystem probe roots")
	}
	if len(p.Fabric.UserspaceCandidates) == 0 {
		t.Fatal("expected fabric userspace candidates")
	}
	if got := fabricUserspaceNameFromModule("ofi/libfabric/1.22.0"); got != "libfabric" {
		t.Fatalf("fabricUserspaceNameFromModule(libfabric) = %q, want libfabric", got)
	}
	if !mpiPolicyPlatformOwned("cray-mpich") {
		t.Fatal("expected cray-mpich to be platform-owned")
	}
	if stringSliceContains(compilerEnvKeys("cce"), "CRAY_CC_VERSION") {
		t.Fatal("CRAY_CC_VERSION must be a compiler version env key, not a prefix env key")
	}
	if !stringSliceContains(compilerVersionEnvKeys("cce"), "CRAY_CC_VERSION") {
		t.Fatal("expected CCE version env key from policy")
	}
	if stringSliceContains(mpiEnvKeys("cray-mpich"), "CRAY_MPICH_VERSION") {
		t.Fatal("CRAY_MPICH_VERSION must be an MPI version env key, not a prefix env key")
	}
	if !stringSliceContains(mpiVersionEnvKeys("cray-mpich"), "CRAY_MPICH_VERSION") {
		t.Fatal("expected Cray MPICH version env key from policy")
	}
	if got := platformProviderFromEnv(p.Platforms["cray-pe"], "PE_ENV", "GNU"); got != "gcc" {
		t.Fatalf("PE_ENV mapping came from policy as %q, want gcc", got)
	}
	if _, ok := systemExternalPolicyByName("cray-libsci"); !ok {
		t.Fatal("expected cray-libsci external candidate policy")
	}
}

func TestROCmSpackComponentsComeFromPolicy(t *testing.T) {
	components := rocmSpackComponents("/opt/rocm-6.0.0")
	if len(components) < 4 {
		t.Fatalf("expected multiple ROCm components, got %#v", components)
	}
	if components[0].Package != "hip" || components[0].Prefix != "/opt/rocm-6.0.0/hip" {
		t.Fatalf("unexpected first ROCm component: %#v", components[0])
	}
	foundLLVM := false
	for _, component := range components {
		if component.Package == "llvm-amdgpu" && component.Prefix == "/opt/rocm-6.0.0" {
			foundLLVM = true
		}
	}
	if !foundLLVM {
		t.Fatalf("expected llvm-amdgpu component at root prefix, got %#v", components)
	}
}

func TestROCmOptionalComponentsRequireEvidence(t *testing.T) {
	prefix := t.TempDir()
	withoutEvidence := rocmSpackComponents(prefix)
	if componentPackagesContain(withoutEvidence, "hipcc") {
		t.Fatalf("hipcc should not be emitted without evidence: %#v", withoutEvidence)
	}

	binDir := filepath.Join(prefix, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "hipcc"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	withEvidence := rocmSpackComponents(prefix)
	if !componentPackagesContain(withEvidence, "hipcc") {
		t.Fatalf("hipcc should be emitted when evidence exists: %#v", withEvidence)
	}
}

func componentPackagesContain(components []model.SpackComponent, want string) bool {
	for _, component := range components {
		if component.Package == want {
			return true
		}
	}
	return false
}

func TestModuleVerificationInputsComeFromPolicy(t *testing.T) {
	envKeys := moduleVerificationEnvKeys()
	for _, want := range []string{"ROCM_PATH", "AOCC_HOME", "CRAYPE_VERSION", "MPICH_DIR", "CRAY_CC_VERSION", "CRAY_MPICH_VERSION", "CRAY_LIBSCI_PREFIX_DIR"} {
		if !stringSliceContains(envKeys, want) {
			t.Fatalf("module verification env keys missing %s: %#v", want, envKeys)
		}
	}
	commands := moduleVerificationCommands()
	for _, want := range []string{"hipcc", "mpicc", "amdclang", "gcc", "fi_info", "ucx_info"} {
		if !stringSliceContains(commands, want) {
			t.Fatalf("module verification commands missing %s: %#v", want, commands)
		}
	}
}

func TestProviderFamilyFromPrefix(t *testing.T) {
	cases := []struct {
		prefix string
		want   string
	}{
		{"/usr", "system"},
		{"/usr/bin", "system"},
		{"/usr/local", "system"},
		{"/opt/site/openmpi", "site"},
	}
	for _, tc := range cases {
		if got := providerFamilyFromPrefix(tc.prefix); got != tc.want {
			t.Fatalf("providerFamilyFromPrefix(%q) = %q, want %q", tc.prefix, got, tc.want)
		}
	}
}
