package commands

import (
	"bytes"
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
