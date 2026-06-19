package probes

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/ray12514/cluster-inspector/internal/model"
)

// FilesystemResult contains shared filesystem candidates.
type FilesystemResult struct {
	Filesystem model.Filesystem
	Evidence   map[string]model.Evidence
}

// ProbeFilesystem identifies install-tree candidates (shared filesystems
// suitable for the Spack install tree), source-cache candidates, and
// buildcache candidates. Checks lock honoring via flock probe.
func ProbeFilesystem() FilesystemResult {
	result := FilesystemResult{
		// Non-nil so yaml.v3 emits an empty array, not `null`. The
		// schema requires `install_tree_candidates` to be an array.
		// minItems is enforced separately at render time; here we just
		// guarantee the shape.
		Filesystem: model.Filesystem{
			InstallTreeCandidates: make([]model.InstallTreeCandidate, 0),
		},
		Evidence: map[string]model.Evidence{},
	}

	candidates := []string{
		"/shared/stack/spack/opt",
		"/apps/spack/opt",
		"/opt/spack/opt",
	}
	for _, path := range candidates {
		if !isDir(path) {
			continue
		}
		candidate := model.InstallTreeCandidate{
			Path:         path,
			Type:         filesystemType(path),
			LocksHonored: locksHonored(path),
		}
		if freeGB := freeGB(path); freeGB >= 0 {
			candidate.FreeGB = &freeGB
		}
		result.Filesystem.InstallTreeCandidates = append(result.Filesystem.InstallTreeCandidates, candidate)
		appendEvidence(result.Evidence, "filesystem.install_tree_candidates."+path, evidence(model.ConfidenceProbed, "existing known install-tree path"))
	}

	if len(result.Filesystem.InstallTreeCandidates) == 0 {
		appendEvidence(result.Evidence, "filesystem.install_tree_candidates", evidence(model.ConfidenceUnknown, "no known install-tree candidates found"))
		return result
	}

	base := strings.TrimSuffix(result.Filesystem.InstallTreeCandidates[0].Path, "/spack/opt")
	if base != result.Filesystem.InstallTreeCandidates[0].Path {
		sourceCache := filepath.Join(base, "spack", "source-cache")
		buildcache := filepath.Join(base, "buildcache")
		if isDir(filepath.Dir(sourceCache)) {
			result.Filesystem.SourceCacheCandidate = sourceCache
			appendEvidence(result.Evidence, "filesystem.source_cache_candidate", evidence(model.ConfidenceInferred, "sibling of install tree"))
		}
		if isDir(filepath.Dir(buildcache)) {
			result.Filesystem.BuildcacheCandidate = buildcache
			appendEvidence(result.Evidence, "filesystem.buildcache_candidate", evidence(model.ConfidenceInferred, "sibling of install tree"))
		}
	}
	return result
}

func filesystemType(path string) string {
	if out, err := run("findmnt", "-n", "-o", "FSTYPE", "--target", path); err == nil && strings.TrimSpace(out) != "" {
		return strings.Fields(out)[0]
	}
	return "unknown"
}

func locksHonored(path string) bool {
	file, err := os.CreateTemp(path, ".cluster-inspector-lock-*")
	if err != nil {
		return false
	}
	name := file.Name()
	defer func() {
		_ = file.Close()
		_ = os.Remove(name)
	}()
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil
}

func freeGB(path string) int {
	if out, err := run("df", "-Pk", path); err == nil {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[len(lines)-1])
			if len(fields) >= 4 {
				kb, err := strconv.Atoi(fields[3])
				if err == nil {
					return kb / 1024 / 1024
				}
			}
		}
	}
	return -1
}
