package probes

import "testing"

func TestRpmPackageVersion(t *testing.T) {
	cases := []struct {
		name string
		out  string
		pkg  string
		want string
	}{
		{
			name: "rdma-core on rhel8",
			out:  "rdma-core-41.0-1.el8.x86_64",
			pkg:  "rdma-core",
			want: "41.0",
		},
		{
			name: "older rhel7 format",
			out:  "rdma-core-22.4-5.el7_8.x86_64",
			pkg:  "rdma-core",
			want: "22.4",
		},
		{
			name: "package missing (rpm prints 'is not installed')",
			out:  "package rdma-core is not installed",
			pkg:  "rdma-core",
			want: "",
		},
		{
			name: "with whitespace",
			out:  "  rdma-core-29.0-1.el8.x86_64  ",
			pkg:  "rdma-core",
			want: "29.0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rpmPackageVersion(tc.out, tc.pkg); got != tc.want {
				t.Fatalf("rpmPackageVersion(%q, %q) = %q, want %q",
					tc.out, tc.pkg, got, tc.want)
			}
		})
	}
}
