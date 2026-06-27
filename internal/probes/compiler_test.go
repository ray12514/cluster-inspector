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

func TestCompilerNameFromProviderPrefixedModule(t *testing.T) {
	cases := []struct {
		module string
		want   string
	}{
		{"gcc/13", "gcc"},
		{"gnu/gcc/13", "gcc"},
		{"cray/gcc-native/13", "gcc"},
		{"amd/aocc/4.2", "aocc"},
		{"oneapi/compiler/2024.2", "oneapi"},
		{"nvidia/nvhpc/25.3", "nvhpc"},
		{"cray/PrgEnv-gnu", "gcc"},
		{"unknown/1.0", ""},
	}
	for _, tc := range cases {
		t.Run(tc.module, func(t *testing.T) {
			if got := compilerNameFromModule(tc.module); got != tc.want {
				t.Fatalf("compilerNameFromModule(%q) = %q, want %q", tc.module, got, tc.want)
			}
		})
	}
}
