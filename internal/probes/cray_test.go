package probes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCrayCompilerFlavor(t *testing.T) {
	cases := []struct {
		peEnv string
		want  string
	}{
		{"GNU", "gcc"},
		{"CRAY", "cce"},
		{"AOCC", "aocc"},
		{"INTEL", "intel"},
		{"AMD", "rocmcc"},
		{"NVIDIA", "nvhpc"},
		{"", "unknown"},
		{"BOGUS", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.peEnv, func(t *testing.T) {
			t.Setenv("PE_ENV", tc.peEnv)
			if got := crayCompilerFlavor(); got != tc.want {
				t.Fatalf("PE_ENV=%q: crayCompilerFlavor() = %q, want %q",
					tc.peEnv, got, tc.want)
			}
		})
	}
}

func TestLatestChildVersion(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"8.1.27", "8.1.28", "8.1.29", "8.1.30-rc1"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	got := latestChildVersion(dir)
	// String comparison: "8.1.30-rc1" sorts after "8.1.29".
	if got != "8.1.30-rc1" {
		t.Fatalf("latestChildVersion: got %q, want 8.1.30-rc1", got)
	}

	// Empty directory yields empty string, not a panic.
	empty := t.TempDir()
	if got := latestChildVersion(empty); got != "" {
		t.Fatalf("latestChildVersion on empty dir: got %q, want \"\"", got)
	}

	// Non-existent path yields empty string.
	if got := latestChildVersion(filepath.Join(dir, "no-such-thing")); got != "" {
		t.Fatalf("latestChildVersion on missing dir: got %q, want \"\"", got)
	}
}

func TestFirstNonEmptyEnv(t *testing.T) {
	t.Setenv("UNSET_A_FOR_TEST", "")
	t.Setenv("UNSET_B_FOR_TEST", "")
	t.Setenv("SET_FOR_TEST", "the-value")

	if got := firstNonEmptyEnv("UNSET_A_FOR_TEST", "SET_FOR_TEST", "UNSET_B_FOR_TEST"); got != "the-value" {
		t.Fatalf("firstNonEmptyEnv picked the wrong slot: got %q", got)
	}
	if got := firstNonEmptyEnv("UNSET_A_FOR_TEST", "UNSET_B_FOR_TEST"); got != "" {
		t.Fatalf("firstNonEmptyEnv with all unset: got %q, want \"\"", got)
	}
}
