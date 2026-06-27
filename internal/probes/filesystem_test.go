package probes

import (
	"path/filepath"
	"strings"
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

func TestCandidateInstallTreePathsUsesExplicitCandidateOnlyForChosenPath(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "stack", "spack", "opt")
	t.Setenv("CLUSTER_INSPECTOR_INSTALL_TREE_CANDIDATE", explicit)

	candidates := candidateInstallTreePaths()
	if !stringSliceContains(candidates, explicit) {
		t.Fatalf("candidateInstallTreePaths() missing explicit candidate %q: %#v", explicit, candidates)
	}
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate, "/scratch/") || strings.HasPrefix(candidate, "/local_scratch/") {
			t.Fatalf("install-tree candidates must not be derived from scratch roots: %#v", candidates)
		}
	}
}
