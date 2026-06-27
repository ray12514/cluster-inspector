package probes

import (
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
}

func mergeProviderEvidence(dst map[string]model.Evidence, src map[string]model.Evidence) {
	for key, value := range src {
		dst[key] = value
	}
}
