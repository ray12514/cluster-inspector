package probes

import "testing"

func TestCompilerLooksLikeAOCCPrefix(t *testing.T) {
	cases := []struct {
		prefix string
		want   bool
	}{
		{"/opt/AMD/aocc-compiler-4.2.0", true},
		{"/opt/AOCC/4.2", true},
		{"/usr", false},
		{"/opt/intel", false},
		{"/opt/AMD/rocm-6.0.0", true}, // /amd/ substring matches by design
		{"/cluster/site/AOCC/4.2/bin", true},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.prefix, func(t *testing.T) {
			if got := compilerLooksLikeAOCCPrefix(tc.prefix); got != tc.want {
				t.Fatalf("compilerLooksLikeAOCCPrefix(%q) = %v, want %v",
					tc.prefix, got, tc.want)
			}
		})
	}
}

func TestSamePrefix(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
		cmd    string
		want   bool
	}{
		{
			name:   "binary lives directly under prefix/bin",
			prefix: "/opt/site/openmpi/5.0.9",
			cmd:    "/opt/site/openmpi/5.0.9/bin/mpicc",
			want:   true,
		},
		{
			name:   "binary lives elsewhere",
			prefix: "/opt/site/openmpi/5.0.9",
			cmd:    "/usr/bin/mpicc",
			want:   false,
		},
		{
			name:   "exact prefix match counts",
			prefix: "/opt/AMD/aocc-compiler-4.2.0",
			cmd:    "/opt/AMD/aocc-compiler-4.2.0/bin/clang",
			want:   true,
		},
		{
			name:   "empty command rejected",
			prefix: "/usr",
			cmd:    "",
			want:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := samePrefix(tc.prefix, tc.cmd); got != tc.want {
				t.Fatalf("samePrefix(%q, %q) = %v, want %v",
					tc.prefix, tc.cmd, got, tc.want)
			}
		})
	}
}

func TestActiveCrayPrgEnvCompiler(t *testing.T) {
	cases := []struct {
		peEnv      string
		candidate  string
		wantActive bool
	}{
		{"GNU", "gcc", true},
		{"GNU", "intel", false},
		{"AOCC", "aocc", true},
		{"INTEL", "intel", true},
		{"INTEL", "oneapi", true},
		{"NVIDIA", "nvhpc", true},
		{"NVIDIA", "gcc", false},
		{"", "gcc", false},
	}
	for _, tc := range cases {
		t.Run(tc.peEnv+"_"+tc.candidate, func(t *testing.T) {
			t.Setenv("PE_ENV", tc.peEnv)
			if got := activeCrayPrgEnvCompiler(tc.candidate); got != tc.wantActive {
				t.Fatalf("PE_ENV=%q + %q: got %v, want %v",
					tc.peEnv, tc.candidate, got, tc.wantActive)
			}
		})
	}
}
