package probes

import (
	"strings"

	inspectorhints "github.com/ray12514/cluster-inspector/internal/hints"
	"github.com/ray12514/cluster-inspector/internal/model"
)

// MPIResult contains generic non-Cray MPI externals.
type MPIResult struct {
	MPI      []model.MPIExternal
	Evidence map[string]model.Evidence
}

// ProbeMPI discovers generic MPI implementations (openmpi, mpich,
// mvapich, intel-mpi, etc.) — excludes Cray MPICH, which is handled by
// ProbeCrayPE.
func ProbeMPI() MPIResult {
	return ProbeMPIWithModules(nil, nil)
}

// ProbeMPIWithModules discovers generic non-Cray MPI implementations from
// the active environment and verified module candidates.
func ProbeMPIWithModules(candidates []ModuleCandidate, hints *inspectorhints.Hints) MPIResult {
	result := MPIResult{Evidence: map[string]model.Evidence{}}
	seen := map[string]bool{}
	mpicc := commandPath("mpicc")
	if mpicc != "" && !strings.HasPrefix(mpicc, "/opt/cray") {
		name, version := detectMPINameVersion()
		if name != "" && version != "" {
			appendMPIExternal(&result.MPI, seen, model.MPIExternal{
				Name:       name,
				Provenance: mpiProvenance(mpicc),
				Version:    version,
				Prefix:     prefixFromCommand(mpicc),
				Modules:    []string{},
			})
			appendEvidence(result.Evidence, "mpi."+name, evidence(model.ConfidenceProbed, "mpicc/mpirun on PATH"))
		}
	}
	for _, mpi := range verifiedMPIModules(candidates, hints, result.Evidence) {
		appendMPIExternal(&result.MPI, seen, mpi)
	}
	for _, mpi := range mpiExtras(hints) {
		appendMPIExternal(&result.MPI, seen, mpi)
		appendEvidence(result.Evidence, "mpi.extra."+mpi.Name, evidence(model.ConfidenceInferred, "inspector-hints extras"))
	}
	if len(result.MPI) == 0 {
		appendEvidence(result.Evidence, "mpi", evidence(model.ConfidenceUnknown, "generic MPI not found"))
	}
	return result
}

func verifiedMPIModules(candidates []ModuleCandidate, hints *inspectorhints.Hints, evidenceMap map[string]model.Evidence) []model.MPIExternal {
	moduleHints := inspectorhints.ModuleHints{}
	if hints != nil {
		moduleHints = hints.MPI
	}
	accepted := applyModulePolicy(candidateNamesByCategory(candidates, "mpi"), moduleHints, nil, evidenceMap, "mpi.module_hints")
	out := []model.MPIExternal{}
	for _, module := range accepted {
		if strings.HasPrefix(strings.ToLower(module), "cray-mpich/") {
			continue
		}
		name := mpiNameFromModule(module)
		if name == "" {
			continue
		}
		verification, err := verifyModules([]string{module})
		if err != nil {
			appendVerificationFailure(evidenceMap, "mpi.verify_failed."+module, []string{module}, err)
			continue
		}
		mpi, ok := mpiExternalFromVerification(name, module, verification)
		if !ok {
			appendEvidence(evidenceMap, "mpi.verify_failed."+module, evidence(model.ConfidenceUnknown, "module loaded but MPI prefix unavailable"))
			continue
		}
		out = append(out, mpi)
		appendEvidence(evidenceMap, "mpi.module."+module, evidence(model.ConfidenceProbed, "clean-shell module verification"))
	}
	return out
}

func mpiExternalFromVerification(name, module string, verification moduleVerification) (model.MPIExternal, bool) {
	prefix := firstNonEmptyString(
		verification.Env["MPI_HOME"],
		verification.Env["MPICH_DIR"],
		verification.Env["I_MPI_ROOT"],
		prefixFromCommand(verification.Commands["mpicc"]),
	)
	if prefix == "" || strings.HasPrefix(prefix, "/opt/cray") {
		return model.MPIExternal{}, false
	}
	version := moduleVersion(module)
	if version == "" {
		version = firstVersion(prefix)
	}
	if version == "" {
		version = "unknown"
	}
	return model.MPIExternal{
		Name:       name,
		Provenance: mpiProvenanceFromPrefix(prefix),
		Version:    version,
		Prefix:     prefix,
		Modules:    verification.Modules,
	}, true
}

func mpiNameFromModule(module string) string {
	lower := strings.ToLower(module)
	switch {
	case strings.HasPrefix(lower, "openmpi/"):
		return "openmpi"
	case strings.HasPrefix(lower, "mpich/"):
		return "mpich"
	case strings.HasPrefix(lower, "mvapich/") || strings.HasPrefix(lower, "mvapich2/"):
		return "mvapich"
	case strings.HasPrefix(lower, "intel-mpi/") || strings.HasPrefix(lower, "impi/"):
		return "intel-mpi"
	default:
		return ""
	}
}

func mpiProvenanceFromPrefix(prefix string) string {
	if strings.HasPrefix(prefix, "/usr/") || strings.HasPrefix(prefix, "/bin/") {
		return "system"
	}
	if strings.HasPrefix(prefix, "/opt/cray") || strings.HasPrefix(prefix, "/opt/nvidia") || strings.HasPrefix(prefix, "/opt/rocm") {
		return "vendor_bundled"
	}
	return "site"
}

func mpiExtras(hints *inspectorhints.Hints) []model.MPIExternal {
	if hints == nil {
		return nil
	}
	out := make([]model.MPIExternal, 0, len(hints.Extras.MPI))
	for _, extra := range hints.Extras.MPI {
		out = append(out, model.MPIExternal{
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

func appendMPIExternal(mpis *[]model.MPIExternal, seen map[string]bool, mpi model.MPIExternal) {
	key := mpi.Name + "@" + mpi.Version + ":" + mpi.Prefix
	if seen[key] {
		return
	}
	seen[key] = true
	*mpis = append(*mpis, mpi)
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
	if strings.HasPrefix(mpicc, "/usr/") || strings.HasPrefix(mpicc, "/bin/") {
		return "system"
	}
	return "site"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
