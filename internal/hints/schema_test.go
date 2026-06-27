package hints

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDesignExample(t *testing.T) {
	doc := `schema_version: 1

compilers:
  include:
    - cce/17.0.1
    - gcc-native/13
  exclude_patterns:
    - "gcc-data/*"
    - "gcc-toolset/*"

mpi:
  include:
    - cray-mpich/8.1.29

gpu_toolkits:
  include:
    - rocm/6.0.0
system_externals:
  include:
    - openssl
    - curl

extras:
  compilers:
    - module: aocc/4.2
      name: aocc
      version: "4.2"
      prefix: /opt/AMD/aocc-compiler-4.2
      languages: [c, c++, fortran]
`

	parsed, err := Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if parsed.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", parsed.SchemaVersion)
	}
	assertStrings(t, parsed.Compilers.Include, []string{"cce/17.0.1", "gcc-native/13"})
	assertStrings(t, parsed.Compilers.ExcludePatterns, []string{"gcc-data/*", "gcc-toolset/*"})
	assertStrings(t, parsed.SystemExternals.Include, []string{"openssl", "curl"})
	if len(parsed.Extras.Compilers) != 1 {
		t.Fatalf("len(Extras.Compilers) = %d, want 1", len(parsed.Extras.Compilers))
	}
	compiler := parsed.Extras.Compilers[0]
	if compiler.Module != "aocc/4.2" || compiler.Name != "aocc" || compiler.Prefix != "/opt/AMD/aocc-compiler-4.2" {
		t.Fatalf("unexpected compiler extra: %#v", compiler)
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inspector-hints.yaml")
	if err := os.WriteFile(path, []byte("schema_version: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	parsed, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}
	if parsed.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", parsed.SchemaVersion)
	}
}

func TestParseRejectsUnknownField(t *testing.T) {
	_, err := Parse(strings.NewReader("schema_version: 1\nunknown: true\n"))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestValidateRejectsInvalidSchemaVersion(t *testing.T) {
	_, err := Parse(strings.NewReader("schema_version: 2\n"))
	if err == nil {
		t.Fatal("expected schema version error")
	}
}

func TestValidateRejectsInvalidGlob(t *testing.T) {
	_, err := Parse(strings.NewReader("schema_version: 1\ncompilers:\n  exclude_patterns: ['[']\n"))
	if err == nil {
		t.Fatal("expected invalid glob error")
	}
}

func TestValidateRejectsIncompleteCompilerExtra(t *testing.T) {
	_, err := Parse(strings.NewReader(`schema_version: 1
extras:
  compilers:
    - module: aocc/4.2
      name: aocc
      version: "4.2"
      prefix: relative/path
      languages: [c]
`))
	if err == nil {
		t.Fatal("expected compiler extra prefix error")
	}
}

func TestValidateRejectsInvalidLanguage(t *testing.T) {
	_, err := Parse(strings.NewReader(`schema_version: 1
extras:
  compilers:
    - module: aocc/4.2
      name: aocc
      version: "4.2"
      prefix: /opt/aocc
      languages: [c, python]
`))
	if err == nil {
		t.Fatal("expected invalid language error")
	}
}

func TestValidateMPIExtraProviderFamilyMatchesProfileSchema(t *testing.T) {
	_, err := Parse(strings.NewReader(`schema_version: 1
extras:
  mpi:
    - module: cray-mpich/8.1.29
      name: cray-mpich
      provenance: platform
      version: 8.1.29
      prefix: /opt/cray/pe/mpich/8.1.29
      compiler: cce@17.0.1
`))
	if err != nil {
		t.Fatalf("platform provenance should be accepted: %v", err)
	}
}

func TestValidateRejectsObsoleteMPIExtraProvenance(t *testing.T) {
	_, err := Parse(strings.NewReader(`schema_version: 1
extras:
  mpi:
    - module: cray-mpich/8.1.29
      name: cray-mpich
      provenance: vendor_bundled
      version: 8.1.29
      prefix: /opt/cray/pe/mpich/8.1.29
      compiler: cce@17.0.1
`))
	if err == nil {
		t.Fatal("expected obsolete provenance error")
	}
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
