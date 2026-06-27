package probes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProbeSystemExternalsFindsFocusedCommandBackedPackages(t *testing.T) {
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "openssl"), "#!/bin/sh\necho 'OpenSSL 3.0.7 1 Nov 2022'\n")
	writeExecutable(t, filepath.Join(bin, "curl"), "#!/bin/sh\necho 'curl 8.7.1 libcurl/8.7.1 OpenSSL/3.0.7'\n")
	t.Setenv("PATH", bin)

	result := ProbeSystemExternals()

	if len(result.Externals) != 2 {
		t.Fatalf("expected 2 externals, got %#v", result.Externals)
	}
	if result.Externals[0].Name != "openssl" || result.Externals[0].Version != "3.0.7" {
		t.Fatalf("unexpected openssl external: %#v", result.Externals[0])
	}
	if result.Externals[0].Prefix != bin {
		t.Fatalf("openssl prefix = %q, want %q", result.Externals[0].Prefix, bin)
	}
	if result.Externals[1].Name != "curl" || result.Externals[1].Version != "8.7.1" {
		t.Fatalf("unexpected curl external: %#v", result.Externals[1])
	}
	if result.Externals[1].Detection == nil || result.Externals[1].Detection.Source != "curl --version" {
		t.Fatalf("unexpected curl detection: %#v", result.Externals[1].Detection)
	}
}

func writeExecutable(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}
