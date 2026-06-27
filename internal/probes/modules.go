package probes

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ray12514/cluster-inspector/internal/model"
	"github.com/ray12514/cluster-inspector/internal/resources"
	"gopkg.in/yaml.v3"
)

const unknownModuleCategory = "unknown"

// ModulesResult contains the detected module system and MODULEPATH roots.
type ModulesResult struct {
	ModulesSystem model.ModulesSystem
	ModulePaths   []string
	Candidates    []ModuleCandidate
	Evidence      map[string]model.Evidence
}

// ModuleCandidate is a raw discovered modulefile name plus lightweight
// category guesses. Verification happens later in clean shells.
type ModuleCandidate struct {
	Name       string
	Categories []string
	Source     string
}

type modulePatternFile struct {
	SchemaVersion int                 `yaml:"schema_version"`
	Categories    map[string][]string `yaml:"categories"`
}

// ProbeModules detects the module tool (Lmod vs Tcl) and enumerates
// MODULEPATH plus available modules. Hints filtering lives in
// applyModulePolicy; clean-shell load-and-verify lives in
// verifyModules — both are called from per-category probes
// (ProbeCrayPEWithModules, ProbeCompilersExternalWithModules, etc.).
func ProbeModules() ModulesResult {
	result := ModulesResult{
		ModulePaths: envList("MODULEPATH"),
		Evidence:    map[string]model.Evidence{},
	}

	lmodVersion := os.Getenv("LMOD_VERSION")
	modulesHome := firstNonEmptyEnv("MODULESHOME", "MODULEHOME")
	switch {
	case lmodVersion != "":
		result.ModulesSystem.Tool = "lmod"
		result.ModulesSystem.Version = lmodVersion
		appendEvidence(result.Evidence, "modules_system.tool", evidence(model.ConfidenceProbed, "LMOD_VERSION"))
		appendEvidence(result.Evidence, "modules_system.version", evidence(model.ConfidenceProbed, "LMOD_VERSION"))
	case modulesHome != "":
		result.ModulesSystem.Tool = "tcl"
		appendEvidence(result.Evidence, "modules_system.tool", evidence(model.ConfidenceProbed, "MODULESHOME/MODULEHOME"))
	default:
		result.ModulesSystem.Tool = detectModuleToolFromPath(result.Evidence)
	}

	if result.ModulesSystem.Version == "" {
		result.ModulesSystem.Version = detectModuleVersion(result.ModulesSystem.Tool, result.Evidence)
	}
	if len(result.ModulePaths) > 0 {
		appendEvidence(result.Evidence, "module_paths", evidence(model.ConfidenceProbed, "MODULEPATH"))
	} else {
		appendEvidence(result.Evidence, "module_paths", evidence(model.ConfidenceUnknown, "MODULEPATH unset"))
	}
	result.Candidates = enumerateModuleCandidates(result.ModulePaths)
	if len(result.Candidates) > 0 {
		appendEvidence(result.Evidence, "module_candidates", evidence(model.ConfidenceProbed, "MODULEPATH walk/module avail"))
		if ambiguous := ambiguousModuleCandidates(result.Candidates); len(ambiguous) > 0 {
			appendEvidence(result.Evidence, "module_candidates.ambiguous", evidence(model.ConfidenceInferred, strings.Join(ambiguous, ",")))
		}
	} else {
		appendEvidence(result.Evidence, "module_candidates", evidence(model.ConfidenceUnknown, "no module candidates discovered"))
	}

	return result
}

func detectModuleToolFromPath(evidenceMap map[string]model.Evidence) string {
	if commandPath("lmod") != "" || commandPath("ml") != "" {
		appendEvidence(evidenceMap, "modules_system.tool", evidence(model.ConfidenceInferred, "lmod/ml command on PATH"))
		return "lmod"
	}
	if commandPath("modulecmd") != "" {
		appendEvidence(evidenceMap, "modules_system.tool", evidence(model.ConfidenceInferred, "modulecmd on PATH"))
		return "tcl"
	}
	appendEvidence(evidenceMap, "modules_system.tool", evidence(model.ConfidenceUnknown, "LMOD_VERSION/MODULESHOME/MODULEHOME/modulecmd unavailable"))
	return ""
}

func detectModuleVersion(tool string, evidenceMap map[string]model.Evidence) string {
	if tool == "tcl" && commandPath("modulecmd") != "" {
		if out, err := run("modulecmd", "--version"); err == nil {
			if version := firstVersion(out); version != "" {
				appendEvidence(evidenceMap, "modules_system.version", evidence(model.ConfidenceProbed, "modulecmd --version"))
				return version
			}
		}
	}
	if tool == "lmod" {
		if out, err := run("lmod", "--version"); err == nil {
			if version := firstVersion(out); version != "" {
				appendEvidence(evidenceMap, "modules_system.version", evidence(model.ConfidenceProbed, "lmod --version"))
				return version
			}
		}
	}
	appendEvidence(evidenceMap, "modules_system.version", evidence(model.ConfidenceUnknown, "module version unavailable"))
	return ""
}

func enumerateModuleCandidates(modulePaths []string) []ModuleCandidate {
	seen := map[string]ModuleCandidate{}
	order := []string{}
	add := func(name, source string) {
		name = cleanModuleName(name)
		if name == "" {
			return
		}
		if existing, ok := seen[name]; ok {
			if !strings.Contains(existing.Source, source) {
				existing.Source += "," + source
				seen[name] = existing
			}
			return
		}
		seen[name] = ModuleCandidate{
			Name:       name,
			Categories: classifyModuleName(name),
			Source:     source,
		}
		order = append(order, name)
	}

	for _, modulePath := range modulePaths {
		for _, name := range enumerateModulePath(modulePath) {
			add(name, "modulepath")
		}
	}
	for _, name := range enumerateModuleAvail() {
		add(name, "module-avail")
	}
	sort.Strings(order)
	out := make([]ModuleCandidate, 0, len(order))
	for _, name := range order {
		out = append(out, seen[name])
	}
	return out
}

func enumerateModulePath(root string) []string {
	root = strings.TrimSpace(root)
	if root == "" || !isDir(root) {
		return nil
	}
	out := []string{}
	_ = filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, filePath)
		if err != nil || rel == "." {
			return nil
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if modulePathDepth(rel) > 4 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if module := moduleNameFromRelativePath(rel); module != "" {
			out = append(out, module)
		}
		return nil
	})
	return out
}

func enumerateModuleAvail() []string {
	switch {
	case commandPath("module") != "":
		if out, err := run("module", "avail", "-t"); err == nil {
			return parseModuleAvail(out)
		}
	case commandPath("ml") != "":
		if out, err := run("ml", "-t", "avail"); err == nil {
			return parseModuleAvail(out)
		}
	}
	return nil
}

func parseModuleAvail(out string) []string {
	modules := []string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(stripModuleMarker(line))
		if line == "" || strings.Contains(line, "---") || strings.HasSuffix(line, ":") {
			continue
		}
		for _, field := range strings.Fields(line) {
			field = strings.Trim(strings.TrimSpace(field), ",")
			if cleanModuleName(field) != "" {
				modules = append(modules, field)
			}
		}
	}
	return cleanUniqueModuleNames(modules)
}

func moduleNameFromRelativePath(rel string) string {
	rel = filepath.ToSlash(filepath.Clean(rel))
	base := path.Base(rel)
	if base == ".version" || base == ".modulerc" || base == "module-info" {
		return ""
	}
	ext := path.Ext(rel)
	if ext == ".lua" || ext == ".tcl" {
		rel = strings.TrimSuffix(rel, ext)
	}
	return cleanModuleName(rel)
}

func cleanModuleName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, "(D)")
	name = strings.TrimSuffix(name, "(L)")
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, " ") {
		return ""
	}
	if strings.Contains(name, ":") || strings.Contains(name, "=") {
		return ""
	}
	return name
}

func stripModuleMarker(line string) string {
	line = strings.ReplaceAll(line, "(D)", "")
	line = strings.ReplaceAll(line, "(L)", "")
	return line
}

func cleanUniqueModuleNames(names []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = cleanModuleName(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func modulePathDepth(rel string) int {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == "" {
		return 0
	}
	return strings.Count(rel, "/") + 1
}

func classifyModuleName(name string) []string {
	patterns := loadModulePatterns()
	categories := make([]string, 0, len(patterns.Categories))
	seen := map[string]bool{}
	appendCategory := func(category string) {
		if !seen[category] {
			categories = append(categories, category)
			seen[category] = true
		}
	}
	for _, category := range orderedModuleCategories(patterns.Categories) {
		for _, pattern := range patterns.Categories[category] {
			matched, err := path.Match(pattern, name)
			if err == nil && matched {
				appendCategory(category)
				break
			}
		}
	}
	for _, category := range segmentCategoriesForModule(name) {
		appendCategory(category)
	}
	if len(categories) == 0 {
		return []string{unknownModuleCategory}
	}
	return categories
}

func segmentCategoriesForModule(name string) []string {
	categories := []string{}
	if compilerNameFromModule(name) != "" {
		categories = append(categories, "compiler")
	}
	if mpiNameFromModule(name) != "" {
		categories = append(categories, "mpi")
	}
	if gpuToolkitNameFromModule(name) != "" {
		categories = append(categories, "gpu_toolkit")
	}
	if moduleHasSegment(name, "libfabric", "ucx", "cxi") {
		categories = append(categories, "fabric_userspace")
	}
	for _, platform := range policy().Platforms {
		if moduleHasSegmentPrefix(name, platform.ModulePrefixes...) || moduleHasSegment(name, platform.ModuleSegments...) {
			categories = append(categories, platform.ModuleCategories...)
		}
	}
	return categories
}

func ambiguousModuleCandidates(candidates []ModuleCandidate) []string {
	out := []string{}
	for _, candidate := range candidates {
		categories := []string{}
		for _, category := range candidate.Categories {
			if category != unknownModuleCategory {
				categories = append(categories, category)
			}
		}
		if len(categories) > 1 {
			out = append(out, candidate.Name+":"+strings.Join(categories, "+"))
		}
	}
	return out
}

func loadModulePatterns() modulePatternFile {
	var patterns modulePatternFile
	if err := yaml.Unmarshal(resources.ModulePatterns, &patterns); err != nil || patterns.SchemaVersion != 1 {
		return fallbackModulePatterns()
	}
	return patterns
}

func fallbackModulePatterns() modulePatternFile {
	categories := map[string][]string{
		"compiler":         {},
		"mpi":              {},
		"gpu_toolkit":      {},
		"fabric_userspace": {"libfabric/*", "*/libfabric/*", "ucx/*", "*/ucx/*", "cxi/*", "*/cxi/*"},
	}
	for _, item := range policy().Compilers {
		for _, segment := range item.ModuleSegments {
			categories["compiler"] = append(categories["compiler"], segment+"/*", "*/"+segment+"/*")
		}
		for _, prefix := range item.ModulePrefixes {
			categories["compiler"] = append(categories["compiler"], prefix+"*", "*/"+prefix+"*")
		}
	}
	for _, item := range policy().MPI {
		for _, segment := range item.ModuleSegments {
			categories["mpi"] = append(categories["mpi"], segment+"/*", "*/"+segment+"/*")
		}
	}
	for _, item := range policy().GPUToolkits {
		for _, segment := range item.ModuleSegments {
			categories["gpu_toolkit"] = append(categories["gpu_toolkit"], segment+"/*", "*/"+segment+"/*")
		}
	}
	for category, platform := range policy().Platforms {
		if len(platform.ModuleCategories) > 0 {
			category = platform.ModuleCategories[0]
		}
		for _, segment := range platform.ModuleSegments {
			categories[category] = append(categories[category], segment+"/*", "*/"+segment+"/*")
		}
		for _, prefix := range platform.ModulePrefixes {
			categories[category] = append(categories[category], prefix+"*", "*/"+prefix+"*")
		}
	}
	return modulePatternFile{
		SchemaVersion: 1,
		Categories:    categories,
	}
}

func orderedModuleCategories(categories map[string][]string) []string {
	preferred := []string{"compiler", "mpi", "gpu_toolkit", "fabric_userspace", "cray_pe"}
	out := make([]string, 0, len(categories))
	seen := map[string]bool{}
	for _, category := range preferred {
		if _, ok := categories[category]; ok {
			out = append(out, category)
			seen[category] = true
		}
	}
	preferredCount := len(out)
	for category := range categories {
		if !seen[category] {
			out = append(out, category)
		}
	}
	sort.Strings(out[preferredCount:])
	return out
}
