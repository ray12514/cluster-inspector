package probes

import (
	"os"
	"strings"

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
	if path := commandPath("fi_info"); path != "" {
		version := ""
		if out, err := run("fi_info", "--version"); err == nil {
			version = firstVersion(out)
		}
		if version != "" {
			userspace = append(userspace, model.NamedPrefixVersioned{Name: "libfabric", Version: version, Prefix: prefixFromCommand(path)})
			appendEvidence(evidenceMap, "fabric.userspace.libfabric", evidence(model.ConfidenceProbed, "fi_info --version"))
		}
	}
	if path := commandPath("ucx_info"); path != "" {
		version := ""
		if out, err := run("ucx_info", "-v"); err == nil {
			version = firstVersion(out)
		}
		if version != "" {
			userspace = append(userspace, model.NamedPrefixVersioned{Name: "ucx", Version: version, Prefix: prefixFromCommand(path)})
			appendEvidence(evidenceMap, "fabric.userspace.ucx", evidence(model.ConfidenceProbed, "ucx_info -v"))
		}
	}
	return userspace
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
