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
