package probes

import (
	"os"
	"strings"

	inspectorhints "github.com/ray12514/cluster-inspector/internal/hints"
	"github.com/ray12514/cluster-inspector/internal/model"
)

// FabricResult contains detected fabric facts.
type FabricResult struct {
	Fabric   model.Fabric
	Evidence map[string]model.Evidence
}

// ProbeFabric detects the fabric stack: type (slingshot/IB/RoCE/OmniPath/
// ethernet), generation marker, kernel/userspace drivers, and userspace
// libraries (libfabric, UCX).
func ProbeFabric() FabricResult {
	return ProbeFabricWithModules(nil, nil)
}

// ProbeFabricWithModules detects fabric facts from active commands, package
// managers, common roots, and verified fabric userspace modules.
func ProbeFabricWithModules(candidates []ModuleCandidate, hints *inspectorhints.Hints) FabricResult {
	result := FabricResult{
		Fabric: model.Fabric{
			Type:    "ethernet",
			Drivers: []model.NamedPrefixVersioned{},
		},
		Evidence: map[string]model.Evidence{},
	}

	switch {
	case hasCXIEvidence():
		result.Fabric.Type = "slingshot"
		result.Fabric.Generation = "cxi"
		appendEvidence(result.Evidence, "fabric.type", evidence(model.ConfidenceProbed, "CXI device evidence"))
		appendEvidence(result.Evidence, "fabric.generation", evidence(model.ConfidenceProbed, "CXI device evidence"))
	case hasInfiniBandEvidence():
		result.Fabric.Type = "infiniband"
		appendEvidence(result.Evidence, "fabric.type", evidence(model.ConfidenceProbed, "/sys/class/infiniband"))
	default:
		appendEvidence(result.Evidence, "fabric.type", evidence(model.ConfidenceInferred, "no CXI/InfiniBand evidence found"))
	}

	result.Fabric.Drivers = probeFabricDrivers(result.Evidence)
	result.Fabric.Userspace = probeFabricUserspace(result.Evidence)
	applyVerifiedFabricUserspaceModules(&result.Fabric.Userspace, candidates, hints, result.Evidence)
	applyFabricUserspaceExtras(&result.Fabric.Userspace, hints, result.Evidence)
	return result
}

func hasCXIEvidence() bool {
	return firstExistingPolicyRoot(policy().Fabric.CXIDevicePaths) != "" || firstExistingPolicyRoot(policy().Fabric.CXIUserspaceRoots) != ""
}

func hasInfiniBandEvidence() bool {
	entries, err := os.ReadDir(policy().Fabric.InfiniBandClassPath)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") {
			return true
		}
	}
	return false
}

func probeFabricDrivers(evidenceMap map[string]model.Evidence) []model.NamedPrefixVersioned {
	// Always return a non-nil slice so the schema's `type: array`
	// constraint passes even when nothing is detected. yaml.v3 emits a
	// nil slice as `null`, which the schema rejects.
	drivers := make([]model.NamedPrefixVersioned, 0)
	if out, err := run("rpm", "-q", "rdma-core"); err == nil {
		if version := rpmPackageVersion(out, "rdma-core"); version != "" {
			drivers = append(drivers, model.NamedPrefixVersioned{Name: "rdma-core", Version: version, Prefix: "/usr"})
			appendEvidence(evidenceMap, "fabric.drivers.rdma-core", evidence(model.ConfidenceProbed, "rpm -q rdma-core"))
		}
	} else if out, err := run("dpkg-query", "-W", "-f=${Version}", "rdma-core"); err == nil && strings.TrimSpace(out) != "" {
		drivers = append(drivers, model.NamedPrefixVersioned{Name: "rdma-core", Version: strings.TrimSpace(out), Prefix: "/usr"})
		appendEvidence(evidenceMap, "fabric.drivers.rdma-core", evidence(model.ConfidenceProbed, "dpkg-query rdma-core"))
	}
	if root := firstExistingPolicyRoot(policy().Fabric.CXIUserspaceRoots); root != "" {
		version := latestChildVersion(root)
		if version == "" {
			version = "unknown"
		}
		drivers = append(drivers, model.NamedPrefixVersioned{Name: "cxi", Version: version, Prefix: root})
		appendEvidence(evidenceMap, "fabric.drivers.cxi", evidence(model.ConfidenceProbed, root))
	}
	return drivers
}

func probeFabricUserspace(evidenceMap map[string]model.Evidence) []model.NamedPrefixVersioned {
	// Non-nil even when empty (see probeFabricDrivers).
	userspace := make([]model.NamedPrefixVersioned, 0)
	seen := map[string]bool{}
	for _, candidate := range policy().Fabric.UserspaceCandidates {
		if item, ok := probeFabricUserspaceCandidate(candidate); ok {
			appendFabricUserspace(&userspace, seen, item)
			appendEvidence(evidenceMap, "fabric.userspace."+candidate.Name, evidence(model.ConfidenceProbed, "discovery policy"))
		}
	}
	return userspace
}

func probeFabricUserspaceCandidate(candidate fabricUserspacePolicy) (model.NamedPrefixVersioned, bool) {
	prefix := firstNonEmptyEnv(candidate.Env...)
	if prefix == "" {
		for _, command := range candidate.Commands {
			if path := commandPath(command); path != "" {
				prefix = prefixFromCommand(path)
				break
			}
		}
	}
	if prefix == "" {
		prefix = firstExistingVersionedRoot(candidate.Roots...)
	}
	if prefix == "" {
		return model.NamedPrefixVersioned{}, false
	}
	version := fabricUserspaceVersion(candidate, prefix)
	if version == "" {
		version = firstVersion(prefix)
	}
	if version == "" {
		version = "unknown"
	}
	return model.NamedPrefixVersioned{Name: candidate.Name, Version: version, Prefix: prefix}, true
}

func fabricUserspaceVersion(candidate fabricUserspacePolicy, prefix string) string {
	for _, versionCommand := range candidate.VersionCommands {
		if len(versionCommand) == 0 {
			continue
		}
		name := versionCommand[0]
		args := versionCommand[1:]
		if !commandIsAvailableForPrefix(name, prefix) {
			continue
		}
		if out, err := run(name, args...); err == nil {
			if version := firstVersion(out); version != "" {
				return version
			}
		}
	}
	return ""
}

func commandIsAvailableForPrefix(name string, prefix string) bool {
	path := commandPath(name)
	return path != "" && (prefix == "" || samePrefix(prefix, path))
}

func applyVerifiedFabricUserspaceModules(userspace *[]model.NamedPrefixVersioned, candidates []ModuleCandidate, hints *inspectorhints.Hints, evidenceMap map[string]model.Evidence) {
	moduleHints := inspectorhints.ModuleHints{}
	if hints != nil {
		moduleHints = hints.FabricUserspace
	}
	accepted := applyModulePolicy(candidateNamesByCategory(candidates, "fabric_userspace"), moduleHints, nil, evidenceMap, "fabric.userspace.module_hints")
	seen := fabricUserspaceSeen(*userspace)
	for _, module := range accepted {
		name := fabricUserspaceNameFromModule(module)
		if name == "" {
			continue
		}
		verification, err := verifyModules([]string{module})
		if err != nil {
			appendVerificationFailure(evidenceMap, "fabric.userspace.verify_failed."+module, []string{module}, err)
			continue
		}
		item, ok := fabricUserspaceFromVerification(name, module, verification)
		if !ok {
			appendEvidence(evidenceMap, "fabric.userspace.verify_failed."+module, evidence(model.ConfidenceUnknown, "module loaded but fabric userspace prefix unavailable"))
			continue
		}
		appendFabricUserspace(userspace, seen, item)
		appendEvidence(evidenceMap, "fabric.userspace."+name+".module."+module, evidence(model.ConfidenceProbed, "clean-shell module verification"))
	}
}

func fabricUserspaceFromVerification(name, module string, verification moduleVerification) (model.NamedPrefixVersioned, bool) {
	candidate, ok := fabricUserspacePolicyByName(name)
	if !ok {
		return model.NamedPrefixVersioned{}, false
	}
	prefix := firstNonEmptyString(fabricUserspaceEnvValues(candidate, verification)...)
	prefix = firstNonEmptyString(prefix, fabricUserspaceCommandPrefix(candidate, verification))
	if prefix == "" {
		return model.NamedPrefixVersioned{}, false
	}
	version := moduleVersion(module)
	if version == "" {
		version = firstVersion(prefix)
	}
	if version == "" {
		version = "unknown"
	}
	return model.NamedPrefixVersioned{Name: name, Version: version, Prefix: prefix}, true
}

func fabricUserspaceNameFromModule(module string) string {
	for _, candidate := range policy().Fabric.UserspaceCandidates {
		if moduleHasSegment(module, candidate.ModuleSegments...) {
			return candidate.Name
		}
	}
	return ""
}

func fabricUserspacePolicyByName(name string) (fabricUserspacePolicy, bool) {
	for _, candidate := range policy().Fabric.UserspaceCandidates {
		if candidate.Name == name {
			return candidate, true
		}
	}
	return fabricUserspacePolicy{}, false
}

func fabricUserspaceEnvValues(candidate fabricUserspacePolicy, verification moduleVerification) []string {
	values := make([]string, 0, len(candidate.Env))
	for _, env := range candidate.Env {
		values = append(values, verification.Env[env])
	}
	return values
}

func fabricUserspaceCommandPrefix(candidate fabricUserspacePolicy, verification moduleVerification) string {
	for _, command := range candidate.Commands {
		if prefix := prefixFromCommand(verification.Commands[command]); prefix != "" {
			return prefix
		}
	}
	return ""
}

func applyFabricUserspaceExtras(userspace *[]model.NamedPrefixVersioned, hints *inspectorhints.Hints, evidenceMap map[string]model.Evidence) {
	if hints == nil {
		return
	}
	seen := fabricUserspaceSeen(*userspace)
	for _, extra := range hints.Extras.FabricUserspace {
		item := model.NamedPrefixVersioned{Name: extra.Name, Version: extra.Version, Prefix: extra.Prefix}
		appendFabricUserspace(userspace, seen, item)
		appendEvidence(evidenceMap, "fabric.userspace.extra."+extra.Name, evidence(model.ConfidenceInferred, "inspector-hints extras"))
	}
}

func appendFabricUserspace(userspace *[]model.NamedPrefixVersioned, seen map[string]bool, item model.NamedPrefixVersioned) {
	key := item.Name + "@" + item.Version + ":" + item.Prefix
	if seen[key] {
		return
	}
	seen[key] = true
	*userspace = append(*userspace, item)
}

func fabricUserspaceSeen(userspace []model.NamedPrefixVersioned) map[string]bool {
	seen := map[string]bool{}
	for _, item := range userspace {
		seen[item.Name+"@"+item.Version+":"+item.Prefix] = true
	}
	return seen
}

func rpmPackageVersion(out, name string) string {
	out = strings.TrimSpace(out)
	out = strings.TrimPrefix(out, name+"-")
	return firstVersion(out)
}

func latestChildVersion(path string) string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return ""
	}
	best := ""
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() > best {
			best = entry.Name()
		}
	}
	return best
}
