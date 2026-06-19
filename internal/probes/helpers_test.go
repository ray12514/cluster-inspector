package probes

import "testing"

func TestFirstVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain dotted", "gcc-13.3.0", "13.3.0"},
		{"openmpi banner", "Open MPI v5.0.9", "5.0.9"},
		{"glibc 2-part", "ldd (GNU libc) 2.34", "2.34"},
		{"absent", "no version here", ""},
		{"two-part with prefix", "AOCC version 4.2.0", "4.2.0"},
		{"only single integer", "version 12", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstVersion(tc.in); got != tc.want {
				t.Fatalf("firstVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitVersion(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantMajor int
		wantMinor *int
	}{
		{"dotted", "8.9", 8, intPtr(9)},
		{"hyphenated SLES", "15-SP5", 15, intPtr(5)},
		{"major only", "9", 9, nil},
		{"empty", "", 0, nil},
		{"non-numeric", "rolling", 0, nil},
		{"three-part keeps first two", "12.4.1", 12, intPtr(4)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			major, minor := splitVersion(tc.in)
			if major != tc.wantMajor {
				t.Fatalf("major: got %d, want %d", major, tc.wantMajor)
			}
			if (minor == nil) != (tc.wantMinor == nil) {
				t.Fatalf("minor presence: got %v, want %v", minor, tc.wantMinor)
			}
			if minor != nil && tc.wantMinor != nil && *minor != *tc.wantMinor {
				t.Fatalf("minor: got %d, want %d", *minor, *tc.wantMinor)
			}
		})
	}
}

func TestPrefixFromCommand(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bin layout", "/opt/site/openmpi/5.0.9/bin/mpicc", "/opt/site/openmpi/5.0.9"},
		{"sbin layout", "/usr/sbin/foo", "/usr"},
		{"no bin segment", "/opt/AMD/aocc/bin/clang", "/opt/AMD/aocc"},
		{"relative path", "bin/foo", ""},
		{"empty", "", ""},
		{"command in /usr/bin", "/usr/bin/gcc", "/usr"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := prefixFromCommand(tc.in); got != tc.want {
				t.Fatalf("prefixFromCommand(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCommandSource(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		args []string
		want string
	}{
		{"no args", "ldd", nil, "ldd"},
		{"one arg", "ldd", []string{"--version"}, "ldd --version"},
		{"many args", "df", []string{"-Pk", "/shared"}, "df -Pk /shared"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := commandSource(tc.cmd, tc.args...); got != tc.want {
				t.Fatalf("commandSource(%q, %v) = %q, want %q", tc.cmd, tc.args, got, tc.want)
			}
		})
	}
}

func TestAppendEvidenceNilSafe(t *testing.T) {
	// Nil destinations should be silently skipped — never panic.
	appendEvidence(nil, "anything", evidence("probed", "src"))
	got := map[string]string{}
	_ = got
}

func TestFirstExistingDir(t *testing.T) {
	// Real filesystem touch points: /tmp exists everywhere we ship.
	if firstExistingDir("/no-such-thing-here", "/tmp") != "/tmp" {
		t.Fatalf("expected /tmp to be returned as the first existing dir")
	}
	if firstExistingDir("/no-such-thing", "/also-not-here") != "" {
		t.Fatalf("expected empty string when no candidate exists")
	}
}

func intPtr(v int) *int { return &v }
