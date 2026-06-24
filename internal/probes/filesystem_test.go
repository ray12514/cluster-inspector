package probes

import (
	"path/filepath"
	"testing"
)

func TestNearestExistingDir(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "stack", "spack", "opt")

	if got := nearestExistingDir(nested); got != root {
		t.Fatalf("nearestExistingDir(%q) = %q, want %q", nested, got, root)
	}
	if got := nearestExistingDir("relative/path"); got != "" {
		t.Fatalf("nearestExistingDir(relative) = %q, want empty", got)
	}
}
