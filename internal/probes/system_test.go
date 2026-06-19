package probes

import (
	"strings"
	"testing"
)

func TestDeriveSystemFamily(t *testing.T) {
	cases := []struct {
		name   string
		osName string
		major  int
		cray   bool
		want   string
	}{
		{"cray + rhel", "rhel", 8, true, "cray-rhel"},
		{"cray + sles", "sles", 15, true, "cray-sles"},
		{"cray + ubuntu (fallback)", "ubuntu", 22, true, "cray-ubuntu"},
		{"rhel 9", "rhel", 9, false, "linux-rhel9"},
		{"rhel no major", "rhel", 0, false, "linux-rhel"},
		{"rocky 8 (rhel-compat)", "rocky", 8, false, "linux-rhel8"},
		{"almalinux 9 (rhel-compat)", "almalinux", 9, false, "linux-rhel9"},
		{"centos 7 (rhel-compat)", "centos", 7, false, "linux-rhel7"},
		{"sles 15", "sles", 15, false, "linux-sles"},
		{"ubuntu", "ubuntu", 22, false, "linux-ubuntu"},
		{"unknown distro gets generic linux-<x>", "exoticos", 0, false, "linux-exoticos"},
		{"empty os yields empty family", "", 0, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveSystemFamily(tc.osName, tc.major, tc.cray); got != tc.want {
				t.Fatalf("deriveSystemFamily(%q, %d, %v) = %q, want %q",
					tc.osName, tc.major, tc.cray, got, tc.want)
			}
		})
	}
}

func TestSanitizeName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "login01", "login01"},
		{"with dots", "cray01.local", "cray01.local"},
		{"with underscore", "node_a", "node_a"},
		{"whitespace becomes hyphen", "node a", "node-a"},
		{"slashes become hyphens", "weird/name/here", "weird-name-here"},
		{"leading/trailing whitespace trimmed", "  trim-me\n", "trim-me"},
		{"all-junk becomes local", "@@@", "local"},
		{"empty becomes local", "", "local"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeName(tc.in); got != tc.want {
				t.Fatalf("sanitizeName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseOSRelease(t *testing.T) {
	cases := []struct {
		name           string
		body           string
		wantID         string
		wantVersionID  string
		wantPrettyName string
	}{
		{
			name: "rhel 8",
			body: `NAME="Red Hat Enterprise Linux"
ID="rhel"
VERSION_ID="8.9"
PRETTY_NAME="Red Hat Enterprise Linux 8.9 (Ootpa)"
`,
			wantID:         "rhel",
			wantVersionID:  "8.9",
			wantPrettyName: "Red Hat Enterprise Linux 8.9 (Ootpa)",
		},
		{
			name: "sles 15 SP5",
			body: `ID="sles"
VERSION_ID="15-SP5"
PRETTY_NAME="SUSE Linux Enterprise Server 15 SP5"
`,
			wantID:         "sles",
			wantVersionID:  "15-SP5",
			wantPrettyName: "SUSE Linux Enterprise Server 15 SP5",
		},
		{
			name: "comments and blank lines tolerated",
			body: `# leading comment

ID=ubuntu
VERSION_ID="22.04"
`,
			wantID:        "ubuntu",
			wantVersionID: "22.04",
		},
		{
			name:          "uppercase ID is lowercased",
			body:          "ID=ROCKY\nVERSION_ID=\"9.4\"\n",
			wantID:        "rocky",
			wantVersionID: "9.4",
		},
		{
			name: "missing required keys yield zero values",
			body: `NAME="Some Linux"
`,
			wantID: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, versionID, pretty := parseOSRelease(strings.NewReader(tc.body))
			if id != tc.wantID {
				t.Errorf("ID: got %q, want %q", id, tc.wantID)
			}
			if versionID != tc.wantVersionID {
				t.Errorf("VERSION_ID: got %q, want %q", versionID, tc.wantVersionID)
			}
			if pretty != tc.wantPrettyName {
				t.Errorf("PRETTY_NAME: got %q, want %q", pretty, tc.wantPrettyName)
			}
		})
	}
}
