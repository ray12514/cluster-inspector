package probes

import (
	"reflect"
	"testing"

	"github.com/ray12514/cluster-inspector/internal/model"
)

func TestMergeProviderInventoryDeduplicatesSameCompilerInstance(t *testing.T) {
	dst := ProviderInventoryResult{Evidence: map[string]model.Evidence{}}

	mergeProviderInventory(&dst, ProviderInventoryResult{
		CompilerProviders: []model.CompilerProvider{{
			Name:           "aocc",
			Version:        "5.1.0",
			Prefix:         "/opt/AMD/aocc-compiler-5.1.0",
			ProviderFamily: "site",
			Languages:      []string{"c", "c++"},
			Modules:        []string{"amd/aocc/5.1.0"},
		}},
	})
	mergeProviderInventory(&dst, ProviderInventoryResult{
		CompilerProviders: []model.CompilerProvider{{
			Name:           "aocc",
			Version:        "5.1.0",
			Prefix:         "/opt/AMD/aocc-compiler-5.1.0",
			ProviderFamily: "site",
			Languages:      []string{"fortran", "c"},
			Modules:        []string{"aocc/5.1.0"},
		}},
	})

	if got := len(dst.CompilerProviders); got != 1 {
		t.Fatalf("len(CompilerProviders) = %d, want 1: %#v", got, dst.CompilerProviders)
	}
	provider := dst.CompilerProviders[0]
	if !reflect.DeepEqual(provider.Languages, []string{"c", "c++", "fortran"}) {
		t.Fatalf("Languages = %#v, want c/c++/fortran", provider.Languages)
	}
	if !reflect.DeepEqual(provider.Modules, []string{"amd/aocc/5.1.0", "aocc/5.1.0"}) {
		t.Fatalf("Modules = %#v, want merged module evidence", provider.Modules)
	}
}

func TestMergeProviderInventoryKeepsDifferentCompilerPrefixesSeparate(t *testing.T) {
	dst := ProviderInventoryResult{Evidence: map[string]model.Evidence{}}

	mergeProviderInventory(&dst, ProviderInventoryResult{
		CompilerProviders: []model.CompilerProvider{{
			Name:           "aocc",
			Version:        "5.1.0",
			Prefix:         "/opt/AMD/aocc-compiler-5.1.0",
			ProviderFamily: "site",
			Languages:      []string{"c"},
		}},
	})
	mergeProviderInventory(&dst, ProviderInventoryResult{
		CompilerProviders: []model.CompilerProvider{{
			Name:           "aocc",
			Version:        "5.1.0",
			Prefix:         "/p/app/aocc/aocc-compiler-5.1.0",
			ProviderFamily: "site",
			Languages:      []string{"c"},
		}},
	})

	if got := len(dst.CompilerProviders); got != 2 {
		t.Fatalf("len(CompilerProviders) = %d, want 2: %#v", got, dst.CompilerProviders)
	}
}

func TestMergeProviderInventoryDeduplicatesSameMPIInstance(t *testing.T) {
	dst := ProviderInventoryResult{Evidence: map[string]model.Evidence{}}

	mergeProviderInventory(&dst, ProviderInventoryResult{
		MPIProviders: []model.MPIProvider{{
			Name:           "openmpi",
			Version:        "5.0.3",
			ProviderFamily: "site",
			Prefix:         "/opt/site/openmpi/5.0.3",
			Compiler:       "gcc@13.3.0",
			Modules:        []string{"gcc/openmpi/5.0.3"},
		}},
	})
	mergeProviderInventory(&dst, ProviderInventoryResult{
		MPIProviders: []model.MPIProvider{{
			Name:           "openmpi",
			Version:        "5.0.3",
			ProviderFamily: "site",
			Prefix:         "/opt/site/openmpi/5.0.3",
			Compiler:       "gcc@13.3.0",
			Modules:        []string{"openmpi/5.0.3"},
		}},
	})

	if got := len(dst.MPIProviders); got != 1 {
		t.Fatalf("len(MPIProviders) = %d, want 1: %#v", got, dst.MPIProviders)
	}
	if !reflect.DeepEqual(dst.MPIProviders[0].Modules, []string{"gcc/openmpi/5.0.3", "openmpi/5.0.3"}) {
		t.Fatalf("Modules = %#v, want merged module evidence", dst.MPIProviders[0].Modules)
	}
}

func TestMergeProviderInventoryKeepsDifferentMPICompilerBindingsSeparate(t *testing.T) {
	dst := ProviderInventoryResult{Evidence: map[string]model.Evidence{}}

	mergeProviderInventory(&dst, ProviderInventoryResult{
		MPIProviders: []model.MPIProvider{{
			Name:           "openmpi",
			Version:        "5.0.3",
			ProviderFamily: "site",
			Prefix:         "/opt/site/openmpi/5.0.3",
			Compiler:       "gcc@13.3.0",
		}},
	})
	mergeProviderInventory(&dst, ProviderInventoryResult{
		MPIProviders: []model.MPIProvider{{
			Name:           "openmpi",
			Version:        "5.0.3",
			ProviderFamily: "site",
			Prefix:         "/opt/site/openmpi/5.0.3",
			Compiler:       "aocc@5.1.0",
		}},
	})

	if got := len(dst.MPIProviders); got != 2 {
		t.Fatalf("len(MPIProviders) = %d, want 2: %#v", got, dst.MPIProviders)
	}
}
