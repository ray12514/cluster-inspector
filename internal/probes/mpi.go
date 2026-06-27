package probes

import (
	"strings"

	inspectorhints "github.com/ray12514/cluster-inspector/internal/hints"
	"github.com/ray12514/cluster-inspector/internal/model"
)

// MPIResult contains generic MPI provider facts.
type MPIResult struct {
	MPIProviders []model.MPIProvider
	Evidence     map[string]model.Evidence
}

type mpiExternal struct {
	Name       string
	Provenance string
	Version    string
	Prefix     string
	Compiler   string
	Modules    []string
}

// ProbeMPI discovers generic MPI implementations (openmpi, mpich,
// mvapich, intel-mpi, etc.). Platform MPI providers are discovered by their
// platform probe and also emitted as generic mpi_providers.
func ProbeMPI() MPIResult {
	return ProbeMPIWithModules(nil, nil)
}

// ProbeMPIWithModules discovers generic non-Cray MPI implementations from
// the active environment and verified module candidates.
func ProbeMPIWithModules(candidates []ModuleCandidate, hints *inspectorhints.Hints) MPIResult {
	result := MPIResult{Evidence: map[string]model.Evidence{}}
	seen := map[string]bool{}
	mpicc := commandPath("mpicc")
	if mpicc != "" && !platformOwnsPrefix(mpicc) {
		name, version := detectMPINameVersion()
		if name != "" && version != "" {
			appendMPIProvider(&result.MPIProviders, seen, mpiProviderFromExternal(mpiExternal{
				Name:       name,
				Provenance: mpiProvenance(mpicc),
				Version:    version,
				Prefix:     prefixFromCommand(mpicc),
				Modules:    []string{},
			}))
			appendEvidence(result.Evidence, "mpi_providers."+name, evidence(model.ConfidenceProbed, "mpicc/mpirun on PATH"))
		}
	}
	for _, mpi := range verifiedMPIModules(candidates, hints, result.Evidence) {
		appendMPIProvider(&result.MPIProviders, seen, mpiProviderFromExternal(mpi))
	}
	for _, mpi := range mpiExtras(hints) {
		appendMPIProvider(&result.MPIProviders, seen, mpiProviderFromExternal(mpi))
		appendEvidence(result.Evidence, "mpi_providers.extra."+mpi.Name, evidence(model.ConfidenceInferred, "inspector-hints extras"))
	}
	if len(result.MPIProviders) == 0 {
		appendEvidence(result.Evidence, "mpi_providers", evidence(model.ConfidenceUnknown, "generic MPI not found"))
	}
	return result
}

func verifiedMPIModules(candidates []ModuleCandidate, hints *inspectorhints.Hints, evidenceMap map[string]model.Evidence) []mpiExternal {
	moduleHints := inspectorhints.ModuleHints{}
	if hints != nil {
		moduleHints = hints.MPI
	}
	accepted := applyModulePolicy(candidateNamesByCategory(candidates, "mpi"), moduleHints, nil, evidenceMap, "mpi_providers.module_hints")
	out := []mpiExternal{}
	for _, module := range accepted {
		name := mpiNameFromModule(module)
		if mpiPolicyPlatformOwned(name) {
			continue
		}
		if name == "" {
			continue
		}
		verification, err := verifyModules([]string{module})
		if err != nil {
			appendVerificationFailure(evidenceMap, "mpi_providers.verify_failed."+module, []string{module}, err)
			continue
		}
		mpi, ok := mpiExternalFromVerification(name, module, verification)
		if !ok {
			appendEvidence(evidenceMap, "mpi_providers.verify_failed."+module, evidence(model.ConfidenceUnknown, "module loaded but MPI prefix unavailable"))
			continue
		}
		out = append(out, mpi)
		appendEvidence(evidenceMap, "mpi_providers.module."+module, evidence(model.ConfidenceProbed, "clean-shell module verification"))
	}
	return out
}

func mpiExternalFromVerification(name, module string, verification moduleVerification) (mpiExternal, bool) {
	prefix := firstNonEmptyString(mpiEnvValues(name, verification)...)
	prefix = firstNonEmptyString(prefix, prefixFromCommand(verification.Commands["mpicc"]))
	if prefix == "" || platformOwnsPrefix(prefix) {
		return mpiExternal{}, false
	}
	version := moduleVersion(module)
	if version == "" {
		version = firstVersion(prefix)
	}
	if version == "" {
		version = "unknown"
	}
	return mpiExternal{
		Name:       name,
		Provenance: mpiProvenanceFromPrefix(prefix),
		Version:    version,
		Prefix:     prefix,
		Modules:    verification.Modules,
	}, true
}

func mpiEnvValues(name string, verification moduleVerification) []string {
	item, ok := mpiPolicyByName(name)
	if !ok {
		return nil
	}
	values := make([]string, 0, len(item.Env))
	for _, env := range item.Env {
		values = append(values, verification.Env[env])
	}
	return values
}

func mpiNameFromModule(module string) string {
	for _, item := range policy().MPI {
		for _, segment := range moduleSegments(module) {
			if stringSliceContains(item.ModuleSegments, segment) {
				return item.Name
			}
		}
	}
	return ""
}

func mpiProvenanceFromPrefix(prefix string) string {
	return providerFamilyFromPrefix(prefix)
}

func mpiExtras(hints *inspectorhints.Hints) []mpiExternal {
	if hints == nil {
		return nil
	}
	out := make([]mpiExternal, 0, len(hints.Extras.MPI))
	for _, extra := range hints.Extras.MPI {
		out = append(out, mpiExternal{
			Name:       extra.Name,
			Provenance: extra.Provenance,
			Version:    extra.Version,
			Prefix:     extra.Prefix,
			Compiler:   extra.Compiler,
			Modules:    []string{extra.Module},
		})
	}
	return out
}

func mpiProviderFromExternal(mpi mpiExternal) model.MPIProvider {
	family := mpi.Provenance
	if family != "system" && family != "site" {
		family = providerFamilyFromPrefix(mpi.Prefix)
	}
	return model.MPIProvider{
		Name:           mpi.Name,
		Version:        mpi.Version,
		ProviderFamily: family,
		Prefix:         mpi.Prefix,
		Compiler:       mpi.Compiler,
		Modules:        mpi.Modules,
	}
}

func appendMPIProvider(providers *[]model.MPIProvider, seen map[string]bool, provider model.MPIProvider) {
	key := provider.Name + "@" + provider.Version + ":" + provider.Prefix
	if seen[key] {
		return
	}
	seen[key] = true
	*providers = append(*providers, provider)
}

func detectMPINameVersion() (string, string) {
	if commandPath("ompi_info") != "" {
		if out, err := run("ompi_info", "--parsable", "--all"); err == nil && strings.Contains(strings.ToLower(out), "open mpi") {
			if version := firstVersion(out); version != "" {
				return "openmpi", version
			}
		}
	}
	if out, err := run("mpirun", "--version"); err == nil {
		lower := strings.ToLower(out)
		version := firstVersion(out)
		switch {
		case strings.Contains(lower, "open mpi"):
			return "openmpi", version
		case strings.Contains(lower, "mpich"):
			return "mpich", version
		case strings.Contains(lower, "mvapich"):
			return "mvapich", version
		case strings.Contains(lower, "intel"):
			return "intel-mpi", version
		}
	}
	if out, err := run("mpichversion"); err == nil {
		if version := firstVersion(out); version != "" {
			return "mpich", version
		}
	}
	return "", ""
}

func mpiProvenance(mpicc string) string {
	// /usr/local is convention for operator-installed software, not
	// distro packages — treat it as site, not system.
	if strings.HasPrefix(mpicc, "/usr/local/") {
		return "site"
	}
	return providerFamilyFromPrefix(mpicc)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mpiPolicyPlatformOwned(name string) bool {
	item, ok := mpiPolicyByName(name)
	return ok && item.PlatformOwned
}
