package probes

import "testing"

func TestMpiProvenance(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/usr/bin/mpicc", "system"},
		{"/usr/local/bin/mpicc", "site"},
		{"/bin/mpicc", "system"},
		{"/opt/site/openmpi/5.0.9-aocc-4.2.0/bin/mpicc", "site"},
		{"/shared/sw/openmpi/4.1.6/bin/mpicc", "site"},
		{"", "site"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := mpiProvenance(tc.path); got != tc.want {
				t.Fatalf("mpiProvenance(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestMPINameFromProviderPrefixedModule(t *testing.T) {
	cases := []struct {
		module string
		want   string
	}{
		{"openmpi/5.0.9", "openmpi"},
		{"amd/openmpi/4.5.6", "openmpi"},
		{"gcc/mpich/4.2.2", "mpich"},
		{"compiler/mvapich2/3.0", "mvapich"},
		{"oneapi/impi/2021.13", "intel-mpi"},
		{"cray/cray-mpich/8.1.29", "cray-mpich"},
		{"unknown/1.0", ""},
	}
	for _, tc := range cases {
		t.Run(tc.module, func(t *testing.T) {
			if got := mpiNameFromModule(tc.module); got != tc.want {
				t.Fatalf("mpiNameFromModule(%q) = %q, want %q", tc.module, got, tc.want)
			}
		})
	}
}
