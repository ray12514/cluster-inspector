package hints

import (
	"fmt"
	"path"
	"strings"
)

// RejectedCandidate records why a discovered module candidate was not
// allowed through the hints filter.
type RejectedCandidate struct {
	Module  string
	Reason  string
	Pattern string
}

// ApplyResult captures the deterministic output of one hints filtering
// pass. Diagnostics can expose Rejected and MissingIncludes later without
// recomputing policy decisions.
type ApplyResult struct {
	Accepted        []string
	Rejected        []RejectedCandidate
	MissingIncludes []string
	Extras          []string
}

// Apply narrows a candidate list to those that pass an `include` filter
// (when non-empty), then drops anything matching `exclude_patterns`, then
// appends `extras`. Used per category (compilers, mpi, gpu_toolkits,
// fabric_userspace).
func Apply(candidates []string, moduleHints ModuleHints, extras []string) (ApplyResult, error) {
	if err := validateModuleHints("hints", moduleHints); err != nil {
		return ApplyResult{}, err
	}

	cleanCandidates := cleanUniqueStrings(candidates)
	includeSet := stringSet(moduleHints.Include)
	candidateSet := stringSet(cleanCandidates)
	result := ApplyResult{}
	acceptedSet := map[string]bool{}

	for _, candidate := range cleanCandidates {
		if len(includeSet) > 0 && !includeSet[candidate] {
			result.Rejected = append(result.Rejected, RejectedCandidate{
				Module: candidate,
				Reason: "not included",
			})
			continue
		}
		matchedPattern, err := firstMatchingPattern(candidate, moduleHints.ExcludePatterns)
		if err != nil {
			return ApplyResult{}, err
		}
		if matchedPattern != "" {
			result.Rejected = append(result.Rejected, RejectedCandidate{
				Module:  candidate,
				Reason:  "excluded",
				Pattern: matchedPattern,
			})
			continue
		}
		result.Accepted = append(result.Accepted, candidate)
		acceptedSet[candidate] = true
	}

	for _, include := range cleanUniqueStrings(moduleHints.Include) {
		if !candidateSet[include] {
			result.MissingIncludes = append(result.MissingIncludes, include)
		}
	}

	for _, extra := range cleanUniqueStrings(extras) {
		if acceptedSet[extra] {
			continue
		}
		result.Accepted = append(result.Accepted, extra)
		result.Extras = append(result.Extras, extra)
		acceptedSet[extra] = true
	}
	return result, nil
}

func cleanUniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func stringSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range cleanUniqueStrings(values) {
		set[value] = true
	}
	return set
}

func firstMatchingPattern(candidate string, patterns []string) (string, error) {
	for _, pattern := range patterns {
		matched, err := path.Match(pattern, candidate)
		if err != nil {
			return "", fmt.Errorf("invalid exclude pattern %q: %w", pattern, err)
		}
		if matched {
			return pattern, nil
		}
	}
	return "", nil
}
