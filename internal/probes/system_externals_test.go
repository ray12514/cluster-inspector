package probes

import (
	"os"
	"path/filepath"
	"testing"

	inspectorhints "github.com/ray12514/cluster-inspector/internal/hints"
)

func TestProbeSystemExternalsFindsFocusedCommandBackedPackages(t *testing.T) {
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "openssl"), "#!/bin/sh\necho 'OpenSSL 3.0.7 1 Nov 2022'\n")
	writeExecutable(t, filepath.Join(bin, "curl"), "#!/bin/sh\necho 'curl 8.7.1 libcurl/8.7.1 OpenSSL/3.0.7'\n")
	t.Setenv("PATH", bin)

	result := ProbeSystemExternals(nil)

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

func TestProbeSystemExternalsHonorsHintsFocus(t *testing.T) {
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "openssl"), "#!/bin/sh\necho 'OpenSSL 3.0.7 1 Nov 2022'\n")
	writeExecutable(t, filepath.Join(bin, "curl"), "#!/bin/sh\necho 'curl 8.7.1 libcurl/8.7.1 OpenSSL/3.0.7'\n")
	t.Setenv("PATH", bin)

	result := ProbeSystemExternals(&inspectorhints.Hints{
		SchemaVersion: 1,
		SystemExternals: inspectorhints.ModuleHints{
			Include: []string{"curl"},
		},
	})

	if len(result.Externals) != 1 {
		t.Fatalf("expected 1 external, got %#v", result.Externals)
	}
	if result.Externals[0].Name != "curl" {
		t.Fatalf("unexpected focused external: %#v", result.Externals[0])
	}
}

func TestProbeSystemExternalsEmitsPolicyBackedLibSci(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("CRAY_LIBSCI_PREFIX_DIR", "/opt/cray/pe/libsci/24.03.0")
	t.Setenv("CRAY_LIBSCI_VERSION", "24.03.0")

	result := ProbeSystemExternals(&inspectorhints.Hints{
		SchemaVersion: 1,
		SystemExternals: inspectorhints.ModuleHints{
			Include: []string{"cray-libsci"},
		},
	})

	if len(result.Externals) != 1 {
		t.Fatalf("expected 1 external, got %#v", result.Externals)
	}
	external := result.Externals[0]
	if external.Name != "cray-libsci" || external.Version != "24.03.0" {
		t.Fatalf("unexpected libsci external: %#v", external)
	}
	if external.ProviderFamily != "platform" {
		t.Fatalf("libsci provider_family = %q, want platform", external.ProviderFamily)
	}
	if len(external.Modules) != 1 || external.Modules[0] != "cray-libsci/24.03.0" {
		t.Fatalf("unexpected libsci modules: %#v", external.Modules)
	}
}

func writeExecutable(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}
