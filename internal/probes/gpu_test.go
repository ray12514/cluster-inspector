package probes

import "testing"

func TestNvidiaArch(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"H100", "9.0\n", "sm_90"},
		{"A100", "8.0", "sm_80"},
		{"H100 multi-line same gpu", "9.0\n9.0\n", "sm_90"},
		{"trailing comma stripped", "9.0,", "sm_90"},
		{"surrounding whitespace", "  9.0  ", "sm_90"},
		{"non-decimal returns empty", "ninepoint0", ""},
		{"empty returns empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nvidiaArch(tc.in); got != tc.want {
				t.Fatalf("nvidiaArch(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestAmdArch(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "rocminfo MI250X agent block",
			in: `*******
Agent 2
*******
  Name:                    gfx90a
  Marketing Name:          AMD Instinct MI250X
`,
			want: "gfx90a",
		},
		{
			name: "rocminfo MI300A",
			in:   "Name: gfx942",
			want: "gfx942",
		},
		{
			name: "no gfx present",
			in:   "Name: amdgcn-amd-amdhsa",
			want: "",
		},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := amdArch(tc.in); got != tc.want {
				t.Fatalf("amdArch(...) = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFilepathJoinRefusesEmpty(t *testing.T) {
	// filepathJoin returns "" if any element is empty — guards against
	// constructing paths like "/bin//nvcc" when CUDA_HOME is unset.
	if got := filepathJoin("/usr/local", "bin", "nvcc"); got != "/usr/local/bin/nvcc" {
		t.Fatalf("filepathJoin: got %q", got)
	}
	if got := filepathJoin("/usr/local", "", "nvcc"); got != "" {
		t.Fatalf("filepathJoin with empty middle should return empty, got %q", got)
	}
	if got := filepathJoin("", "bin", "nvcc"); got != "" {
		t.Fatalf("filepathJoin with empty head should return empty, got %q", got)
	}
}

func TestGPUToolkitNameFromProviderPrefixedModule(t *testing.T) {
	cases := []struct {
		module string
		want   string
	}{
		{"rocm/6.0.0", "rocm"},
		{"amd/rocm/6.0.0", "rocm"},
		{"cuda/12.4", "cudatoolkit"},
		{"nvidia/cuda/12.4", "cudatoolkit"},
		{"nvidia/cudatoolkit/12.4", "cudatoolkit"},
		{"nvidia/nvhpc/25.3", "nvhpc"},
		{"unknown/1.0", ""},
	}
	for _, tc := range cases {
		t.Run(tc.module, func(t *testing.T) {
			if got := gpuToolkitNameFromModule(tc.module); got != tc.want {
				t.Fatalf("gpuToolkitNameFromModule(%q) = %q, want %q", tc.module, got, tc.want)
			}
		})
	}
}
