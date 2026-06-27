package probes

import (
	"path/filepath"
	"testing"

	"github.com/ray12514/cluster-inspector/internal/model"
)

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

func TestProbeFabricUserspaceUsesDiscoveryPolicyCommands(t *testing.T) {
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "fi_info"), "#!/bin/sh\necho 'libfabric: 1.22.0'\n")
	writeExecutable(t, filepath.Join(bin, "ucx_info"), "#!/bin/sh\necho '# UCT version=1.16.0'\n")
	t.Setenv("PATH", bin)

	result := ProbeFabric()

	if !fabricUserspaceContains(result.Fabric.Userspace, "libfabric", "1.22.0", bin) {
		t.Fatalf("missing policy-discovered libfabric userspace: %#v", result.Fabric.Userspace)
	}
	if !fabricUserspaceContains(result.Fabric.Userspace, "ucx", "1.16.0", bin) {
		t.Fatalf("missing policy-discovered ucx userspace: %#v", result.Fabric.Userspace)
	}
}

func TestFabricUserspaceFromVerificationUsesModuleAndCommandPrefix(t *testing.T) {
	verification := moduleVerification{
		Commands: map[string]string{
			"fi_info": "/opt/ofi/libfabric/1.22.0/bin/fi_info",
		},
		Env: map[string]string{},
	}

	item, ok := fabricUserspaceFromVerification("libfabric", "libfabric/1.22.0", verification)
	if !ok {
		t.Fatal("expected module verification to produce libfabric userspace")
	}
	if item.Name != "libfabric" || item.Version != "1.22.0" || item.Prefix != "/opt/ofi/libfabric/1.22.0" {
		t.Fatalf("unexpected fabric userspace item: %#v", item)
	}
}

func fabricUserspaceContains(items []model.NamedPrefixVersioned, name string, version string, prefix string) bool {
	for _, item := range items {
		if item.Name == name && item.Version == version && item.Prefix == prefix {
			return true
		}
	}
	return false
}
