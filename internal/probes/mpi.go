package probes

import (
	"strings"

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
//
// TODO: Phase 4 — add module candidate enumeration, hints filtering, and
// clean-shell verification.
func ProbeMPI() MPIResult {
	result := MPIResult{Evidence: map[string]model.Evidence{}}
	mpicc := commandPath("mpicc")
	if mpicc == "" || strings.HasPrefix(mpicc, "/opt/cray") {
		appendEvidence(result.Evidence, "mpi", evidence(model.ConfidenceUnknown, "generic mpicc not found"))
		return result
	}

	name, version := detectMPINameVersion()
	if name == "" || version == "" {
		appendEvidence(result.Evidence, "mpi", evidence(model.ConfidenceUnknown, "mpicc found but MPI identity/version unavailable"))
		return result
	}

	result.MPI = append(result.MPI, model.MPIExternal{
		Name:       name,
		Provenance: mpiProvenance(mpicc),
		Version:    version,
		Prefix:     prefixFromCommand(mpicc),
		Modules:    []string{},
	})
	appendEvidence(result.Evidence, "mpi."+name, evidence(model.ConfidenceProbed, "mpicc/mpirun on PATH"))
	return result
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
