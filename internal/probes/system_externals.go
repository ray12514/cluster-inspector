package probes

import (
	"path/filepath"
	"strings"

	inspectorhints "github.com/ray12514/cluster-inspector/internal/hints"
	"github.com/ray12514/cluster-inspector/internal/model"
)

// SystemExternalsResult contains focused ordinary package external candidates.
type SystemExternalsResult struct {
	Externals []model.SystemExternal
	Evidence  map[string]model.Evidence
}

// ProbeSystemExternals discovers a small focused set of ordinary package
// externals that stack defaults commonly want to use from the host OS.
func ProbeSystemExternals(hints *inspectorhints.Hints) SystemExternalsResult {
	result := SystemExternalsResult{
		Externals: []model.SystemExternal{},
		Evidence:  map[string]model.Evidence{},
	}
	for _, name := range focusedSystemExternalNames(hints, result.Evidence) {
		if external, ok := probeFocusedSystemExternal(name, result.Evidence); ok {
			result.Externals = append(result.Externals, external)
		}
	}
	return result
}

func focusedSystemExternalNames(hints *inspectorhints.Hints, evidenceMap map[string]model.Evidence) []string {
	moduleHints := inspectorhints.ModuleHints{}
	if hints != nil {
		moduleHints = hints.SystemExternals
	}
	defaultFocus := policy().SystemExternals.DefaultFocus
	candidates := append([]string{}, defaultFocus...)
	candidates = append(candidates, moduleHints.Include...)
	result, err := inspectorhints.Apply(candidates, moduleHints, nil)
	if err != nil {
		appendEvidence(evidenceMap, "system_externals.hints", evidence(model.ConfidenceUnknown, err.Error()))
		return defaultFocus
	}
	if len(result.Rejected) > 0 || len(result.MissingIncludes) > 0 {
		appendEvidence(evidenceMap, "system_externals.hints", evidence(model.ConfidenceInferred, "inspector-hints system_externals filter applied"))
	}
	return result.Accepted
}

func probeFocusedSystemExternal(
	name string,
	evidenceMap map[string]model.Evidence,
) (model.SystemExternal, bool) {
	switch name {
	case "openssl":
		if path := commandPath("openssl"); path != "" {
			if out, err := run("openssl", "version"); err == nil {
				if version := firstVersion(out); version != "" {
					appendEvidence(evidenceMap, "system_externals.openssl", evidence(model.ConfidenceProbed, "openssl version"))
					return systemExternal("openssl", version, prefixFromCommand(path), "probed", "openssl version"), true
				}
			}
		}
	case "curl":
		if path := commandPath("curl"); path != "" {
			if out, err := run("curl", "--version"); err == nil {
				if version := firstVersion(out); version != "" {
					appendEvidence(evidenceMap, "system_externals.curl", evidence(model.ConfidenceProbed, "curl --version"))
					return systemExternal("curl", version, prefixFromCommand(path), "probed", "curl --version"), true
				}
			}
		}
	}
	if external, ok := probePolicySystemExternal(name, evidenceMap); ok {
		return external, true
	}
	if external, ok := probeSystemPackageManagerExternal(name, evidenceMap); ok {
		return external, true
	}
	appendEvidence(evidenceMap, "system_externals."+name, evidence(model.ConfidenceUnknown, "not found"))
	return model.SystemExternal{}, false
}

func probePolicySystemExternal(
	name string,
	evidenceMap map[string]model.Evidence,
) (model.SystemExternal, bool) {
	item, ok := systemExternalPolicyByName(name)
	if !ok {
		return model.SystemExternal{}, false
	}
	prefix := firstNonEmptyEnv(item.Env...)
	if prefix == "" {
		prefix = firstExistingVersionedRoot(item.Roots...)
	}
	if prefix == "" {
		return model.SystemExternal{}, false
	}
	version := firstNonEmptyEnv(item.VersionEnv...)
	if version == "" {
		version = firstVersion(prefix)
	}
	if version == "" {
		version = "unknown"
	}
	providerFamily := item.ProviderFamily
	if providerFamily == "" {
		providerFamily = providerFamilyFromPrefix(prefix)
	}
	external := systemExternal(name, version, prefix, "probed", "discovery policy")
	external.ProviderFamily = providerFamily
	if item.ModuleTemplate != "" && version != "unknown" {
		external.Modules = []string{strings.ReplaceAll(item.ModuleTemplate, "{version}", version)}
	}
	appendEvidence(evidenceMap, "system_externals."+name, evidence(model.ConfidenceProbed, "discovery policy"))
	return external, true
}

func firstExistingVersionedRoot(roots ...string) string {
	for _, root := range roots {
		if root == "" || !isDir(root) {
			continue
		}
		if version := latestChildVersion(root); version != "" {
			return filepath.Join(root, version)
		}
		return root
	}
	return ""
}

func probeSystemPackageManagerExternal(
	name string,
	evidenceMap map[string]model.Evidence,
) (model.SystemExternal, bool) {
	if out, err := run("rpm", "-q", name); err == nil {
		if version := rpmPackageVersion(out, name); version != "" {
			appendEvidence(evidenceMap, "system_externals."+name, evidence(model.ConfidenceProbed, "rpm -q "+name))
			return systemExternal(name, version, "/usr", "probed", "rpm"), true
		}
	}
	if out, err := run("dpkg-query", "-W", "-f=${Version}", name); err == nil {
		if version := strings.TrimSpace(out); version != "" {
			appendEvidence(evidenceMap, "system_externals."+name, evidence(model.ConfidenceProbed, "dpkg-query "+name))
			return systemExternal(name, version, "/usr", "probed", "dpkg-query"), true
		}
	}
	return model.SystemExternal{}, false
}

func systemExternal(
	name string,
	version string,
	prefix string,
	confidence string,
	source string,
) model.SystemExternal {
	return model.SystemExternal{
		Name:           name,
		Version:        version,
		Prefix:         prefix,
		ProviderFamily: "system",
		Detection:      &model.ExternalDetection{Confidence: confidence, Source: source},
	}
}
