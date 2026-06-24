package probes

import (
	"fmt"
	"strings"

	inspectorhints "github.com/ray12514/cluster-inspector/internal/hints"
	"github.com/ray12514/cluster-inspector/internal/model"
)

func candidateNamesByCategory(candidates []ModuleCandidate, category string) []string {
	names := []string{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if !stringSliceContains(candidate.Categories, category) || seen[candidate.Name] {
			continue
		}
		seen[candidate.Name] = true
		names = append(names, candidate.Name)
	}
	return names
}

func applyModulePolicy(candidates []string, moduleHints inspectorhints.ModuleHints, extras []string, evidenceMap map[string]model.Evidence, evidenceKey string) []string {
	result, err := inspectorhints.Apply(candidates, moduleHints, extras)
	if err != nil {
		appendEvidence(evidenceMap, evidenceKey, evidence(model.ConfidenceUnknown, err.Error()))
		return nil
	}
	parts := []string{fmt.Sprintf("accepted=%d", len(result.Accepted))}
	if len(result.Rejected) > 0 {
		parts = append(parts, fmt.Sprintf("rejected=%d", len(result.Rejected)))
	}
	if len(result.MissingIncludes) > 0 {
		parts = append(parts, "missing_include="+strings.Join(result.MissingIncludes, ","))
	}
	if len(result.Extras) > 0 {
		parts = append(parts, "extras="+strings.Join(result.Extras, ","))
	}
	appendEvidence(evidenceMap, evidenceKey, evidence(model.ConfidenceInferred, strings.Join(parts, "; ")))
	return result.Accepted
}

func appendVerificationFailure(evidenceMap map[string]model.Evidence, key string, modules []string, err error) {
	appendEvidence(evidenceMap, key, evidence(model.ConfidenceUnknown, strings.Join(modules, ",")+": "+err.Error()))
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func moduleVersion(module string) string {
	if _, version, ok := strings.Cut(module, "/"); ok {
		if parsed := firstVersion(version); parsed != "" {
			return parsed
		}
		return version
	}
	return ""
}

func moduleSegments(module string) []string {
	parts := strings.Split(strings.ToLower(module), "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

func moduleHasSegment(module string, names ...string) bool {
	for _, segment := range moduleSegments(module) {
		for _, name := range names {
			if segment == strings.ToLower(name) {
				return true
			}
		}
	}
	return false
}

func moduleHasSegmentPrefix(module string, prefixes ...string) bool {
	for _, segment := range moduleSegments(module) {
		for _, prefix := range prefixes {
			if strings.HasPrefix(segment, strings.ToLower(prefix)) {
				return true
			}
		}
	}
	return false
}
