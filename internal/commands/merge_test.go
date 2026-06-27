package commands

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ray12514/cluster-inspector/internal/model"
	"github.com/ray12514/cluster-inspector/internal/output"
)

func TestMergeFragmentsValidatesGPUNodeTypes(t *testing.T) {
	profile, err := mergeFragments(minimalSystemFragment(), []model.NodeFragment{
		minimalNodeFragment("login", "build_host", nil),
		minimalNodeFragment("cpu_compute", "runtime", nil),
		minimalNodeFragment("gpu_compute_mi250x", "runtime", &model.GPUBlock{
			Vendor:         "amd",
			DriverVersion:  "6.0",
			ToolkitCeiling: "6.0.0",
			ArchTarget:     "gfx90a",
		}),
		minimalNodeFragment("gpu_compute_mi300a", "runtime", &model.GPUBlock{
			Vendor:         "amd",
			DriverVersion:  "6.1",
			ToolkitCeiling: "6.1.0",
			ArchTarget:     "gfx942",
		}),
	})
	if err != nil {
		t.Fatalf("mergeFragments returned error: %v", err)
	}
	if err := model.ValidateProfile(profile); err != nil {
		t.Fatalf("merged profile failed schema validation: %v", err)
	}
	if len(profile.NodeTypes) != 4 {
		t.Fatalf("len(NodeTypes) = %d, want 4", len(profile.NodeTypes))
	}
	if profile.NodeTypes["gpu_compute_mi250x"].GPU.ArchTarget != "gfx90a" {
		t.Fatalf("MI250X arch was not preserved")
	}
	if profile.NodeTypes["gpu_compute_mi300a"].GPU.ArchTarget != "gfx942" {
		t.Fatalf("MI300A arch was not preserved")
	}
}

func TestMergeFragmentsOutputIsByteIdentical(t *testing.T) {
	profile, err := mergeFragments(minimalSystemFragment(), []model.NodeFragment{
		minimalNodeFragment("login", "build_host", nil),
		minimalNodeFragment("cpu_compute", "runtime", nil),
	})
	if err != nil {
		t.Fatalf("mergeFragments returned error: %v", err)
	}

	var first bytes.Buffer
	if err := output.WriteProfile(&first, profile); err != nil {
		t.Fatalf("first WriteProfile: %v", err)
	}
	var second bytes.Buffer
	if err := output.WriteProfile(&second, profile); err != nil {
		t.Fatalf("second WriteProfile: %v", err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatalf("expected byte-identical output for repeated merge writes")
	}
	outputText := first.String()
	if !strings.Contains(outputText, "system_externals:") || !strings.Contains(outputText, "name: openssl") {
		t.Fatalf("expected system externals in profile output:\n%s", outputText)
	}
}

// TestMergeRoundTripIsByteIdentical is the design-doc-strict version of
// the determinism acceptance: re-running the FULL merge → write pipeline
// on the same fragments must produce byte-identical YAML. Catches any
// future regression where mergeFragments grows non-deterministic
// behaviour (e.g., unsorted map iteration that escapes into the output).
func TestMergeRoundTripIsByteIdentical(t *testing.T) {
	systemFragment := minimalSystemFragment()
	nodeFragments := []model.NodeFragment{
		minimalNodeFragment("login", "build_host", nil),
		minimalNodeFragment("cpu_compute", "runtime", nil),
		minimalNodeFragment("gpu_compute_mi250x", "runtime", &model.GPUBlock{
			Vendor:         "amd",
			DriverVersion:  "6.0",
			ToolkitCeiling: "6.0.0",
			ArchTarget:     "gfx90a",
		}),
		minimalNodeFragment("gpu_compute_mi300a", "runtime", &model.GPUBlock{
			Vendor:         "amd",
			DriverVersion:  "6.1",
			ToolkitCeiling: "6.1.0",
			ArchTarget:     "gfx942",
		}),
	}

	render := func() []byte {
		t.Helper()
		profile, err := mergeFragments(systemFragment, nodeFragments)
		if err != nil {
			t.Fatalf("mergeFragments: %v", err)
		}
		var buf bytes.Buffer
		if err := output.WriteProfile(&buf, profile); err != nil {
			t.Fatalf("WriteProfile: %v", err)
		}
		return buf.Bytes()
	}

	first := render()
	for i := 0; i < 5; i++ {
		again := render()
		if !bytes.Equal(first, again) {
			t.Fatalf("merge round-trip %d differs from initial:\n--- first ---\n%s\n--- again ---\n%s",
				i+1, first, again)
		}
	}
}

func TestMergeFragmentsRejectsDuplicateNodeType(t *testing.T) {
	_, err := mergeFragments(minimalSystemFragment(), []model.NodeFragment{
		minimalNodeFragment("login", "build_host", nil),
		minimalNodeFragment("login", "runtime", nil),
	})
	if err == nil {
		t.Fatal("expected duplicate node fragment error")
	}
}

func TestFixtureFragmentsMergeAndValidate(t *testing.T) {
	cases := []struct {
		name  string
		nodes []string
	}{
		{
			name:  "example-cray",
			nodes: []string{"login", "cpu_compute", "gpu_compute_mi250x", "gpu_compute_mi300a"},
		},
		{
			name:  "example-linux",
			nodes: []string{"login", "cpu_compute", "gpu_compute_mi250x", "gpu_compute_h100"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := fixtureDirPath(t, tc.name)
			systemFragment, err := readSystemFragment(filepath.Join(dir, "system.yaml"))
			if err != nil {
				t.Fatalf("readSystemFragment: %v", err)
			}
			nodeFragments := make([]model.NodeFragment, 0, len(tc.nodes))
			for _, node := range tc.nodes {
				fragment, err := readNodeFragment(filepath.Join(dir, "nodes", node+".yaml"))
				if err != nil {
					t.Fatalf("readNodeFragment(%s): %v", node, err)
				}
				nodeFragments = append(nodeFragments, fragment)
			}
			profile, err := mergeFragments(systemFragment, nodeFragments)
			if err != nil {
				t.Fatalf("mergeFragments: %v", err)
			}
			if err := model.ValidateProfile(profile); err != nil {
				t.Fatalf("ValidateProfile: %v", err)
			}
			if errs := validateProfileSemantics(profile); len(errs) > 0 {
				t.Fatalf("validateProfileSemantics: %v", errs)
			}
		})
	}
}

func minimalSystemFragment() model.SystemFragment {
	return model.SystemFragment{
		SchemaVersion: 1,
		System: model.System{
			Name:   "example-linux",
			Family: "linux-rhel9",
		},
		OS: model.OS{
			Name:  "rhel",
			Major: 9,
			Glibc: "2.34",
		},
		Fabric: model.Fabric{
			Type:    "ethernet",
			Drivers: []model.NamedPrefixVersioned{},
		},
		ModulesSystem: model.ModulesSystem{Tool: "lmod"},
		SystemExternals: []model.SystemExternal{{
			Name:           "openssl",
			Version:        "3.0.7",
			Prefix:         "/usr",
			ProviderFamily: "system",
			Variants:       "+shared",
			Detection: &model.ExternalDetection{
				Confidence: "probed",
				Source:     "test fixture",
			},
		}},
		Filesystem: model.Filesystem{
			InstallTreeCandidates: []model.InstallTreeCandidate{{
				Path:         "/shared/stack/spack/opt",
				Type:         "lustre",
				LocksHonored: true,
			}},
		},
	}
}

func minimalNodeFragment(name, role string, gpu *model.GPUBlock) model.NodeFragment {
	return model.NodeFragment{
		Name: name,
		Role: role,
		CPU: model.CPU{
			Detected:  "zen3",
			Preferred: "zen3",
		},
		GPU: gpu,
		BuildStage: []model.BuildStage{{
			Path:            "/tmp",
			Visibility:      "node-local",
			Writable:        true,
			ThroughputClass: "unknown",
		}},
	}
}
