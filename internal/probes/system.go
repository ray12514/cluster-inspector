// Package probes implements per-area probe functions that collect
// observed system facts. Each function returns a fragment slice plus an
// Evidence record per fact.
//
// Shell discipline: any subprocess that invokes a shell uses a non-login
// shell (bash -c '...'), never a login-shell bash or sh invocation. The rule
// is enforced by the shell-discipline grep in Makefile and pre-commit.
// See stack-planning/docs/cluster_inspector_stack_profile_design_v1.md
// § Shell Invocation Discipline.
package probes

import (
	"bufio"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ray12514/cluster-inspector/internal/model"
)

// SystemResult contains identity and OS facts from the local host.
type SystemResult struct {
	System       model.System
	OS           model.OS
	CrayEvidence bool
	Evidence     map[string]model.Evidence
}

// ProbeSystem collects OS identity, glibc version, and hostname identity.
func ProbeSystem(systemName string) SystemResult {
	result := SystemResult{
		System:   model.System{Name: systemName},
		Evidence: map[string]model.Evidence{},
	}
	if result.System.Name == "" {
		result.System.Name = hostnameFallback()
		appendEvidence(result.Evidence, "system.name", evidence(model.ConfidenceInferred, "hostname fallback"))
	} else {
		appendEvidence(result.Evidence, "system.name", evidence(model.ConfidenceInferred, "--system"))
	}

	osID, versionID, prettyName := readOSRelease()
	if osID == "" {
		osID = strings.ToLower(runOrEmpty("uname", "-s"))
		appendEvidence(result.Evidence, "os.name", evidence(model.ConfidenceUnknown, "uname fallback"))
	} else {
		appendEvidence(result.Evidence, "os.name", evidence(model.ConfidenceProbed, "/etc/os-release ID"))
	}
	major, minor := splitVersion(versionID)
	result.OS = model.OS{
		Name:  osID,
		Major: major,
		Minor: minor,
		Glibc: probeGlibc(result.Evidence),
	}
	if prettyName != "" {
		result.System.Description = prettyName
	}

	result.CrayEvidence = detectCrayEvidence()
	result.System.Family = deriveSystemFamily(result.OS.Name, result.OS.Major, result.CrayEvidence)
	appendEvidence(result.Evidence, "system.family", evidence(model.ConfidenceInferred, "os identity + Cray evidence"))

	return result
}

func readOSRelease() (id string, versionID string, prettyName string) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "", "", ""
	}
	defer func() {
		_ = file.Close()
	}()
	return parseOSRelease(file)
}

// parseOSRelease parses an os-release-formatted stream. Split out from
// readOSRelease so tests can exercise the parser without touching
// /etc/os-release.
func parseOSRelease(r io.Reader) (id string, versionID string, prettyName string) {
	scanner := bufio.NewScanner(r)
	values := map[string]string{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[key] = strings.Trim(value, `"`)
	}
	return strings.ToLower(values["ID"]), values["VERSION_ID"], values["PRETTY_NAME"]
}

func probeGlibc(evidenceMap map[string]model.Evidence) string {
	if out, err := run("getconf", "GNU_LIBC_VERSION"); err == nil {
		if version := firstVersion(out); version != "" {
			appendEvidence(evidenceMap, "os.glibc", evidence(model.ConfidenceProbed, commandSource("getconf", "GNU_LIBC_VERSION")))
			return version
		}
	}
	if out, err := run("ldd", "--version"); err == nil {
		if version := firstVersion(out); version != "" {
			appendEvidence(evidenceMap, "os.glibc", evidence(model.ConfidenceProbed, commandSource("ldd", "--version")))
			return version
		}
	}
	appendEvidence(evidenceMap, "os.glibc", evidence(model.ConfidenceUnknown, "getconf/ldd unavailable"))
	return ""
}

func detectCrayEvidence() bool {
	return isDir("/opt/cray/pe") || os.Getenv("CRAYPE_VERSION") != "" || os.Getenv("PE_ENV") != ""
}

func deriveSystemFamily(osName string, major int, cray bool) string {
	switch {
	case cray && osName == "rhel":
		return "cray-rhel"
	case cray && osName == "sles":
		return "cray-sles"
	case cray:
		return "cray-" + osName
	case osName == "rhel" || osName == "rocky" || osName == "almalinux" || osName == "centos":
		if major > 0 {
			return "linux-rhel" + strconvItoa(major)
		}
		return "linux-rhel"
	case osName == "sles":
		return "linux-sles"
	case osName == "ubuntu":
		return "linux-ubuntu"
	case osName != "":
		return "linux-" + osName
	default:
		return ""
	}
}

func hostnameFallback() string {
	if out := runOrEmpty("hostname", "-s"); out != "" {
		return sanitizeName(out)
	}
	if out := runOrEmpty("hostname"); out != "" {
		return sanitizeName(out)
	}
	return "local"
}

func runOrEmpty(name string, args ...string) string {
	out, err := run(name, args...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func sanitizeName(name string) string {
	re := regexp.MustCompile(`[^-_.a-zA-Z0-9]+`)
	name = re.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "local"
	}
	return name
}

func strconvItoa(v int) string {
	return strconv.Itoa(v)
}
