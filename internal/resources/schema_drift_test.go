package resources

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEmbeddedProfileSchemaMatchesCanonical guards the embedded↔canonical seam.
//
// The profile schema is one cross-repo contract. stack-planning owns the
// canonical copy; `make sync-schema` copies it into profile_schema.json so the
// binary is self-contained. This test fails the moment the embedded copy drifts
// from canonical, so a schema edit can't land in stack-planning without being
// synced here (the drift that previously slipped through silently).
//
// Canonical is located via STACK_PLANNING (same var the Makefile uses), else a
// sibling checkout. Skips when stack-planning is not present.
func TestEmbeddedProfileSchemaMatchesCanonical(t *testing.T) {
	canonical := canonicalProfileSchemaPath()
	if canonical == "" {
		t.Skip("stack-planning not found; set STACK_PLANNING or check it out as a sibling")
	}
	want, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatalf("read canonical schema %s: %v", canonical, err)
	}
	got, err := os.ReadFile("profile_schema.json")
	if err != nil {
		t.Fatalf("read embedded schema: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("embedded profile_schema.json drifted from canonical %s; run `make sync-schema`", canonical)
	}
}

func canonicalProfileSchemaPath() string {
	root := os.Getenv("STACK_PLANNING")
	if root == "" {
		// Default dev layout: stack-planning is a sibling of the repo root.
		// Test CWD is the package dir (internal/resources); ../.. is the repo root.
		repoRoot, err := filepath.Abs("../..")
		if err != nil {
			return ""
		}
		root = filepath.Join(filepath.Dir(repoRoot), "stack-planning")
	}
	path := filepath.Join(root, "schemas", "profile-v1.json")
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}
