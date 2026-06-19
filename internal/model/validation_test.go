package model_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"github.com/ray12514/cluster-inspector/internal/model"
)

// fixturesDir resolves the tests/fixtures directory relative to this
// source file, so tests can be run from any working directory.
func fixturesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "tests", "fixtures")
}

// loadYAMLAsJSON reads a YAML file at the given path and re-encodes it
// as a generic Go value the JSON Schema validator can consume directly.
func loadYAMLAsJSON(t *testing.T, path string) any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal yaml %q: %v", path, err)
	}
	// santhosh-tekuri/jsonschema expects a JSON-compatible value tree
	// (map[string]any / []any / primitives). yaml.v3 returns those
	// directly when unmarshalling into an untyped any, so doc is
	// already in the right shape — but we round-trip through JSON to
	// normalise number types (int vs float64) the same way the
	// embedded schema validator sees them.
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("re-marshal yaml as json: %v", err)
	}
	decoded, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode normalised json: %v", err)
	}
	return decoded
}

func TestProfileFixturesValidate(t *testing.T) {
	schema, err := model.CompileProfileSchema()
	if err != nil {
		t.Fatalf("compile embedded schema: %v", err)
	}

	for _, name := range []string{"example-cray", "example-linux"} {
		t.Run(name, func(t *testing.T) {
			fixture := filepath.Join(fixturesDir(t), name, "profile.yaml")
			instance := loadYAMLAsJSON(t, fixture)
			if err := schema.Validate(instance); err != nil {
				t.Fatalf("%s fixture failed schema validation: %v", name, err)
			}
		})
	}
}

// TestEmbeddedSchemaCompiles is a smoke test for the embedded
// profile-v1.json: if the file is missing or syntactically broken the
// compiler returns an error that bubbles up here.
func TestEmbeddedSchemaCompiles(t *testing.T) {
	if _, err := model.CompileProfileSchema(); err != nil {
		t.Fatalf("CompileProfileSchema: %v", err)
	}
}

// TestValidateProfileNilSafe documents the contract that ValidateProfile
// returns an error rather than panicking on a nil profile.
func TestValidateProfileNilSafe(t *testing.T) {
	if err := model.ValidateProfile(nil); err == nil {
		t.Fatal("expected an error for nil profile, got nil")
	}
}
