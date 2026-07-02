package probes

import (
	"sort"
	"strings"

	inspectorhints "github.com/ray12514/cluster-inspector/internal/hints"
	"github.com/ray12514/cluster-inspector/internal/model"
)

// ProviderInventoryResult is the generic provider inventory emitted by all
// provider discovery adapters. Adapter implementations may be platform-aware,
// but this result must stay profile-shape generic.
type ProviderInventoryResult struct {
	CompilerProviders []model.CompilerProvider
	MPIProviders      []model.MPIProvider
	GPUToolkits       *model.GPUToolkitModules
	Evidence          map[string]model.Evidence
}

// ProbeProviderInventory runs the generic Linux adapter first, then augments it
// with platform adapters such as Cray PE when their evidence is present.
func ProbeProviderInventory(candidates []ModuleCandidate, hints *inspectorhints.Hints) ProviderInventoryResult {
	result := ProviderInventoryResult{
		GPUToolkits: &model.GPUToolkitModules{},
		Evidence:    map[string]model.Evidence{},
	}
	mergeProviderInventory(&result, probeLinuxGenericProviders(candidates, hints))
	mergeProviderInventory(&result, probeCrayPEProviders(candidates, hints))
	return result
}

func probeLinuxGenericProviders(candidates []ModuleCandidate, hints *inspectorhints.Hints) ProviderInventoryResult {
	compilers := ProbeCompilersExternalWithModules(candidates, hints)
	mpi := ProbeMPIWithModules(candidates, hints)
	gpu := ProbeGPUToolkitModulesWithModules(candidates, hints)

	result := ProviderInventoryResult{
		CompilerProviders: compilers.CompilerProviders,
		MPIProviders:      mpi.MPIProviders,
		GPUToolkits:       gpu.Toolkits,
		Evidence:          map[string]model.Evidence{},
	}
	mergeProviderEvidence(result.Evidence, compilers.Evidence)
	mergeProviderEvidence(result.Evidence, mpi.Evidence)
	mergeProviderEvidence(result.Evidence, gpu.Evidence)
	return result
}

func probeCrayPEProviders(candidates []ModuleCandidate, hints *inspectorhints.Hints) ProviderInventoryResult {
	cray := ProbeCrayPEWithModules(candidates, hints)
	return ProviderInventoryResult{
		CompilerProviders: cray.CompilerProviders,
		MPIProviders:      cray.MPIProviders,
		Evidence:          cray.Evidence,
	}
}

func mergeProviderInventory(dst *ProviderInventoryResult, src ProviderInventoryResult) {
	dst.CompilerProviders = append(dst.CompilerProviders, src.CompilerProviders...)
	dst.MPIProviders = append(dst.MPIProviders, src.MPIProviders...)
	if src.GPUToolkits != nil {
		if dst.GPUToolkits == nil {
			dst.GPUToolkits = &model.GPUToolkitModules{}
		}
		if src.GPUToolkits.ROCm != nil {
			dst.GPUToolkits.ROCm = src.GPUToolkits.ROCm
		}
		if src.GPUToolkits.CUDAToolkit != nil {
			dst.GPUToolkits.CUDAToolkit = src.GPUToolkits.CUDAToolkit
		}
		if src.GPUToolkits.NVHPC != nil {
			dst.GPUToolkits.NVHPC = src.GPUToolkits.NVHPC
		}
	}
	mergeProviderEvidence(dst.Evidence, src.Evidence)
	dst.CompilerProviders = dedupeCompilerProviders(dst.CompilerProviders)
	dst.MPIProviders = dedupeMPIProviders(dst.MPIProviders)
}

func mergeProviderEvidence(dst map[string]model.Evidence, src map[string]model.Evidence) {
	for key, value := range src {
		dst[key] = value
	}
}

func dedupeCompilerProviders(providers []model.CompilerProvider) []model.CompilerProvider {
	out := make([]model.CompilerProvider, 0, len(providers))
	index := map[string]int{}
	for _, provider := range providers {
		key := compilerProviderInstanceKey(provider)
		if pos, ok := index[key]; ok {
			out[pos] = mergeCompilerProvider(out[pos], provider)
			continue
		}
		index[key] = len(out)
		out = append(out, provider)
	}
	return out
}

func compilerProviderInstanceKey(provider model.CompilerProvider) string {
	return strings.Join([]string{
		provider.Name,
		provider.Version,
		cleanProviderPrefix(provider.Prefix),
	}, "\x00")
}

func mergeCompilerProvider(dst, src model.CompilerProvider) model.CompilerProvider {
	dst.ProviderFamily = preferredProviderFamily(dst.ProviderFamily, src.ProviderFamily)
	if dst.PlatformFamily == "" {
		dst.PlatformFamily = src.PlatformFamily
	}
	dst.Languages = mergeStringLists(dst.Languages, src.Languages)
	dst.Modules = mergeStringLists(dst.Modules, src.Modules)
	if dst.Compilers == nil {
		dst.Compilers = src.Compilers
	}
	return dst
}

func dedupeMPIProviders(providers []model.MPIProvider) []model.MPIProvider {
	out := make([]model.MPIProvider, 0, len(providers))
	index := map[string]int{}
	for _, provider := range providers {
		key := mpiProviderInstanceKey(provider)
		if pos, ok := index[key]; ok {
			out[pos] = mergeMPIProvider(out[pos], provider)
			continue
		}
		index[key] = len(out)
		out = append(out, provider)
	}
	return out
}

func mpiProviderInstanceKey(provider model.MPIProvider) string {
	return strings.Join([]string{
		provider.Name,
		provider.Version,
		cleanProviderPrefix(provider.Prefix),
		provider.Compiler,
		mpiFlavorSignature(provider.Flavors),
	}, "\x00")
}

func mergeMPIProvider(dst, src model.MPIProvider) model.MPIProvider {
	dst.ProviderFamily = preferredProviderFamily(dst.ProviderFamily, src.ProviderFamily)
	if dst.PlatformFamily == "" {
		dst.PlatformFamily = src.PlatformFamily
	}
	dst.Modules = mergeStringLists(dst.Modules, src.Modules)
	if dst.Compatibility == nil {
		dst.Compatibility = src.Compatibility
	} else if src.Compatibility != nil {
		dst.Compatibility.Compilers = mergeStringLists(
			dst.Compatibility.Compilers,
			src.Compatibility.Compilers,
		)
	}
	return dst
}

func cleanProviderPrefix(prefix string) string {
	return strings.TrimRight(strings.TrimSpace(prefix), "/")
}

func preferredProviderFamily(a, b string) string {
	rank := map[string]int{"system": 0, "site": 1, "platform": 2}
	if rank[b] > rank[a] {
		return b
	}
	if a != "" {
		return a
	}
	return b
}

func mergeStringLists(a, b []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(a)+len(b))
	for _, values := range [][]string{a, b} {
		for _, value := range values {
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func mpiFlavorSignature(flavors map[string]model.MPIFlavor) string {
	if len(flavors) == 0 {
		return ""
	}
	keys := make([]string, 0, len(flavors))
	for compiler := range flavors {
		keys = append(keys, compiler)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, compiler := range keys {
		flavor := flavors[compiler]
		parts = append(parts, strings.Join([]string{
			compiler,
			cleanProviderPrefix(flavor.Prefix),
			strings.Join(flavor.Modules, ","),
		}, "="))
	}
	return strings.Join(parts, "|")
}
