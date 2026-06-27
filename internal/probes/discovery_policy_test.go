package probes

import "testing"

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
	if len(rocm.SpackComponents) == 0 {
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
	if !mpiPolicyPlatformOwned("cray-mpich") {
		t.Fatal("expected cray-mpich to be platform-owned")
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

func TestModuleVerificationInputsComeFromPolicy(t *testing.T) {
	envKeys := moduleVerificationEnvKeys()
	for _, want := range []string{"ROCM_PATH", "AOCC_HOME", "CRAYPE_VERSION", "MPICH_DIR"} {
		if !stringSliceContains(envKeys, want) {
			t.Fatalf("module verification env keys missing %s: %#v", want, envKeys)
		}
	}
	commands := moduleVerificationCommands()
	for _, want := range []string{"hipcc", "mpicc", "amdclang", "gcc"} {
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
