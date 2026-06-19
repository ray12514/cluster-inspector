package probes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeCPUTarget(t *testing.T) {
	cases := []struct {
		name  string
		facts cpuFacts
		want  string
	}{
		{
			name: "AMD Milan model name",
			facts: cpuFacts{
				Arch:      "x86_64",
				ModelName: "AMD EPYC 7763 64-Core Processor",
			},
			want: "zen3",
		},
		{
			name: "AMD Genoa model name",
			facts: cpuFacts{
				Arch:      "x86_64",
				ModelName: "AMD EPYC 9654 96-Core Processor",
			},
			want: "zen4",
		},
		{
			name: "AMD family model fallback",
			facts: cpuFacts{
				Arch:   "x86_64",
				Vendor: "AuthenticAMD",
				Family: 25,
				Model:  1,
			},
			want: "zen3",
		},
		{
			name: "x86_64 v3 flags",
			facts: cpuFacts{
				Arch: "x86_64",
				Flags: map[string]bool{
					"avx2": true,
					"bmi2": true,
					"fma":  true,
				},
			},
			want: "x86_64_v3",
		},
		{
			name: "arm64 normalizes to aarch64",
			facts: cpuFacts{
				Arch: "arm64",
			},
			want: "aarch64",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeCPUTarget(tc.facts); got != tc.want {
				t.Fatalf("normalizeCPUTarget(...) = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseCPUKeyValue(t *testing.T) {
	in := strings.NewReader(`Architecture:          x86_64
Vendor ID:             AuthenticAMD
Model name:            AMD EPYC 7763 64-Core Processor
CPU family:            25
Model:                 1
Flags:                 fpu sse4_2 avx2 bmi2 fma
`)
	facts := parseLSCPU(in)
	if facts.Arch != "x86_64" {
		t.Fatalf("Arch = %q, want x86_64", facts.Arch)
	}
	if facts.Vendor != "AuthenticAMD" {
		t.Fatalf("Vendor = %q, want AuthenticAMD", facts.Vendor)
	}
	if facts.Family != 25 || facts.Model != 1 {
		t.Fatalf("family/model = %d/%d, want 25/1", facts.Family, facts.Model)
	}
	if !facts.Flags["avx2"] || !facts.Flags["bmi2"] || !facts.Flags["fma"] {
		t.Fatalf("expected parsed CPU flags, got %#v", facts.Flags)
	}
}

func TestCPUAlternates(t *testing.T) {
	got := cpuAlternates("zen3")
	want := []string{"zen2", "x86_64_v3", "x86_64_v2", "x86_64"}
	if len(got) != len(want) {
		t.Fatalf("len(cpuAlternates) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cpuAlternates[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBuildStageForPathCleansUpProbeFile(t *testing.T) {
	dir := t.TempDir()
	stage := buildStageForPath(dir)
	if stage.Path != dir {
		t.Fatalf("Path = %q, want %q", stage.Path, dir)
	}
	if !stage.Writable {
		t.Fatal("expected temp test directory to be writable")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}
	for _, entry := range entries {
		if matched, _ := filepath.Match(".cluster-inspector-write-*", entry.Name()); matched {
			t.Fatalf("probe file was not cleaned up: %s", entry.Name())
		}
	}
}
