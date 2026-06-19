package probes

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/ray12514/cluster-inspector/internal/model"
)

// NodeResult contains per-node-type facts for the local host.
type NodeResult struct {
	CPU        model.CPU
	GPU        *model.GPUBlock
	BuildStage []model.BuildStage
	Evidence   map[string]model.Evidence
}

type cpuFacts struct {
	Arch      string
	Vendor    string
	ModelName string
	Family    int
	Model     int
	Flags     map[string]bool
}

// ProbeNode runs the per-node-type probes: CPU target detection, GPU
// presence + facts (delegates to ProbeGPU), and build-stage candidate
// discovery (writability, free space, mount options, throughput class).
func ProbeNode() NodeResult {
	result := NodeResult{Evidence: map[string]model.Evidence{}}
	result.CPU = probeCPU(result.Evidence)

	gpu := ProbeGPU()
	result.GPU = gpu.GPU
	for key, value := range gpu.Evidence {
		result.Evidence[key] = value
	}

	result.BuildStage = probeBuildStages(result.Evidence)
	return result
}

func probeCPU(evidenceMap map[string]model.Evidence) model.CPU {
	facts := collectCPUFacts()
	detected := normalizeCPUTarget(facts)
	confidence := model.ConfidenceInferred
	source := "CPU model/flags"
	if facts.ModelName != "" || facts.Vendor != "" || facts.Arch != "" {
		confidence = model.ConfidenceProbed
	}
	if detected == "" {
		detected = "unknown"
		confidence = model.ConfidenceUnknown
		source = "lscpu and /proc/cpuinfo unavailable"
	}
	appendEvidence(evidenceMap, "node.cpu", evidence(confidence, source))
	return model.CPU{
		Detected:   detected,
		Preferred:  detected,
		Alternates: cpuAlternates(detected),
	}
}

func collectCPUFacts() cpuFacts {
	facts := cpuFacts{Flags: map[string]bool{}}
	if out, err := run("lscpu"); err == nil {
		mergeCPUFacts(&facts, parseLSCPU(strings.NewReader(out)))
	}
	if file, err := os.Open("/proc/cpuinfo"); err == nil {
		mergeCPUFacts(&facts, parseCPUInfo(file))
		_ = file.Close()
	}
	if facts.Arch == "" {
		facts.Arch = runtime.GOARCH
	}
	return facts
}

func parseLSCPU(r io.Reader) cpuFacts {
	return parseCPUKeyValue(r)
}

func parseCPUInfo(r io.Reader) cpuFacts {
	return parseCPUKeyValue(r)
}

func parseCPUKeyValue(r io.Reader) cpuFacts {
	facts := cpuFacts{Flags: map[string]bool{}}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "architecture":
			facts.Arch = value
		case "vendor id", "vendor_id":
			facts.Vendor = value
		case "model name", "cpu":
			facts.ModelName = value
		case "cpu family":
			facts.Family = parseCPUInt(value)
		case "model":
			facts.Model = parseCPUInt(value)
		case "flags", "features":
			for _, flag := range strings.Fields(value) {
				facts.Flags[strings.ToLower(flag)] = true
			}
		}
	}
	return facts
}

func mergeCPUFacts(dst *cpuFacts, src cpuFacts) {
	if dst.Arch == "" {
		dst.Arch = src.Arch
	}
	if dst.Vendor == "" {
		dst.Vendor = src.Vendor
	}
	if dst.ModelName == "" {
		dst.ModelName = src.ModelName
	}
	if dst.Family == 0 {
		dst.Family = src.Family
	}
	if dst.Model == 0 {
		dst.Model = src.Model
	}
	if dst.Flags == nil {
		dst.Flags = map[string]bool{}
	}
	for flag := range src.Flags {
		dst.Flags[flag] = true
	}
}

func normalizeCPUTarget(facts cpuFacts) string {
	arch := strings.ToLower(facts.Arch)
	if arch == "amd64" {
		arch = "x86_64"
	}
	if arch == "arm64" {
		arch = "aarch64"
	}
	if arch == "aarch64" {
		return "aarch64"
	}
	if arch != "" && arch != "x86_64" {
		return arch
	}

	nameTarget := cpuTargetFromModelName(facts.ModelName)
	if nameTarget != "" {
		return nameTarget
	}

	vendor := strings.ToLower(facts.Vendor)
	if strings.Contains(vendor, "authenticamd") {
		switch {
		case facts.Family == 25 && facts.Model >= 16:
			return "zen4"
		case facts.Family == 25:
			return "zen3"
		case facts.Family == 23:
			return "zen2"
		}
	}

	if arch == "x86_64" || arch == "" {
		return x86TargetFromFlags(facts.Flags)
	}
	return ""
}

func cpuTargetFromModelName(modelName string) string {
	name := strings.ToLower(modelName)
	switch {
	case strings.Contains(name, "mi300a") || strings.Contains(name, "genoa") || strings.Contains(name, "bergamo") || regexp.MustCompile(`epyc\s+9[0-9]{3}`).MatchString(name):
		return "zen4"
	case strings.Contains(name, "milan") || regexp.MustCompile(`epyc\s+7[0-9]{2}3`).MatchString(name):
		return "zen3"
	case strings.Contains(name, "rome") || regexp.MustCompile(`epyc\s+7[0-9]{2}2`).MatchString(name):
		return "zen2"
	case strings.Contains(name, "sapphire rapids") || regexp.MustCompile(`xeon.*(84[0-9]{2}|64[0-9]{2})`).MatchString(name):
		return "sapphirerapids"
	}
	return ""
}

func x86TargetFromFlags(flags map[string]bool) string {
	if hasAllFlags(flags, "avx512f", "avx512bw", "avx512cd", "avx512dq", "avx512vl") {
		return "x86_64_v4"
	}
	if hasAllFlags(flags, "avx2", "bmi2", "fma") {
		return "x86_64_v3"
	}
	if hasAllFlags(flags, "sse4_2", "popcnt") {
		return "x86_64_v2"
	}
	return "x86_64"
}

func hasAllFlags(flags map[string]bool, names ...string) bool {
	for _, name := range names {
		if !flags[name] {
			return false
		}
	}
	return true
}

func cpuAlternates(target string) []string {
	switch target {
	case "zen4":
		return []string{"zen3", "zen2", "x86_64_v3", "x86_64_v2", "x86_64"}
	case "zen3":
		return []string{"zen2", "x86_64_v3", "x86_64_v2", "x86_64"}
	case "zen2":
		return []string{"x86_64_v3", "x86_64_v2", "x86_64"}
	case "sapphirerapids":
		return []string{"icelake", "skylake_avx512", "x86_64_v4", "x86_64_v3", "x86_64_v2", "x86_64"}
	case "x86_64_v4":
		return []string{"x86_64_v3", "x86_64_v2", "x86_64"}
	case "x86_64_v3":
		return []string{"x86_64_v2", "x86_64"}
	case "x86_64_v2":
		return []string{"x86_64"}
	default:
		return nil
	}
}

func probeBuildStages(evidenceMap map[string]model.Evidence) []model.BuildStage {
	paths := candidateBuildStagePaths()
	stages := make([]model.BuildStage, 0, len(paths))
	for _, path := range paths {
		if !isDir(path) {
			continue
		}
		stages = append(stages, buildStageForPath(path))
	}
	if len(stages) == 0 {
		stages = append(stages, buildStageForPath(os.TempDir()))
	}
	appendEvidence(evidenceMap, "node.build_stage", evidence(model.ConfidenceProbed, "candidate path write/remove checks"))
	return stages
}

func candidateBuildStagePaths() []string {
	user := os.Getenv("USER")
	paths := []string{
		os.Getenv("TMPDIR"),
		os.Getenv("SLURM_TMPDIR"),
		os.Getenv("PBS_JOBFS"),
		os.Getenv("TMP"),
		os.Getenv("TEMP"),
	}
	if user != "" {
		paths = append(paths,
			filepath.Join("/local_scratch", user),
			filepath.Join("/scratch", user),
			filepath.Join("/tmp", user),
		)
	}
	paths = append(paths, "/local_scratch", "/scratch", os.TempDir(), "/tmp", "/var/tmp")
	return cleanPathList(paths)
}

func cleanPathList(paths []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out
}

func buildStageForPath(path string) model.BuildStage {
	stage := model.BuildStage{
		Path:            path,
		Visibility:      buildStageVisibility(path),
		Writable:        pathWritable(path),
		MountOpts:       mountOptions(path),
		ThroughputClass: throughputClass(path),
	}
	if freeGB := freeGB(path); freeGB >= 0 {
		stage.FreeGB = &freeGB
	}
	if freeInodes := freeInodes(path); freeInodes >= 0 {
		stage.FreeInodes = &freeInodes
	}
	return stage
}

func buildStageVisibility(path string) string {
	path = filepath.Clean(path)
	switch {
	case strings.HasPrefix(path, "/local_scratch") || strings.HasPrefix(path, "/lscratch"):
		return "compute-only"
	case path == filepath.Clean(os.Getenv("SLURM_TMPDIR")) && os.Getenv("SLURM_TMPDIR") != "":
		return "compute-only"
	case path == filepath.Clean(os.Getenv("PBS_JOBFS")) && os.Getenv("PBS_JOBFS") != "":
		return "compute-only"
	case path == "/tmp" || path == "/var/tmp" || strings.HasPrefix(path, "/tmp/"):
		return "node-local"
	case strings.HasPrefix(path, "/scratch") || strings.HasPrefix(path, "/shared") || strings.HasPrefix(path, "/gpfs") || strings.HasPrefix(path, "/lustre"):
		return "shared"
	default:
		return "unknown"
	}
}

func pathWritable(path string) bool {
	file, err := os.CreateTemp(path, ".cluster-inspector-write-*")
	if err != nil {
		return false
	}
	name := file.Name()
	closeErr := file.Close()
	removeErr := os.Remove(name)
	return closeErr == nil && removeErr == nil
}

func freeInodes(path string) int {
	if out, err := run("df", "-Pi", path); err == nil {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[len(lines)-1])
			if len(fields) >= 4 {
				inodes, err := strconv.Atoi(fields[3])
				if err == nil {
					return inodes
				}
			}
		}
	}
	return -1
}

func mountOptions(path string) []string {
	out, err := run("findmnt", "-n", "-o", "OPTIONS", "--target", path)
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	parts := strings.Split(strings.TrimSpace(out), ",")
	opts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			opts = append(opts, part)
		}
	}
	sort.Strings(opts)
	return opts
}

func throughputClass(path string) string {
	fsType := strings.ToLower(filesystemType(path))
	path = filepath.Clean(path)
	switch {
	case strings.Contains(path, "local_scratch") || fsType == "tmpfs" || fsType == "xfs" || fsType == "ext4":
		return "fast"
	case fsType == "lustre" || fsType == "gpfs" || fsType == "beegfs" || fsType == "weka":
		return "medium"
	case fsType == "nfs" || fsType == "cifs" || fsType == "smbfs":
		return "slow"
	default:
		return "unknown"
	}
}

func parseCPUInt(value string) int {
	match := regexp.MustCompile(`[0-9]+`).FindString(value)
	if match == "" {
		return 0
	}
	parsed, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return parsed
}
