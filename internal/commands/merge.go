package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ray12514/cluster-inspector/internal/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// toCompilerProviders transforms the probe-detected vendor_cray + generic
// compiler externals into the generic, emitted compiler_providers inventory.
// Cray PE compilers become provider_family=cray-pe; everything else is tagged
// site (or system for /usr prefixes). Probing stays Cray-aware; the emitted
// facts are generic.
func toCompilerProviders(f model.SystemFragment) []model.CompilerProvider {
	out := []model.CompilerProvider{}
	if f.VendorCray != nil {
		langs := []string{"c", "c++", "fortran"}
		blocks := []struct {
			name  string
			block *model.CrayCompilerBlock
		}{
			{"cce", f.VendorCray.CCE}, {"gcc", f.VendorCray.GCC}, {"aocc", f.VendorCray.AOCC},
			{"intel", f.VendorCray.Intel}, {"rocmcc", f.VendorCray.ROCmCC}, {"nvhpc", f.VendorCray.NVHPC},
		}
		for _, nb := range blocks {
			if nb.block == nil {
				continue
			}
			out = append(out, model.CompilerProvider{
				Name: nb.name, Version: nb.block.Version, Prefix: nb.block.Prefix,
				ProviderFamily: "cray-pe", Languages: langs, Modules: nb.block.Modules,
			})
		}
	}
	for _, c := range f.CompilersExternal {
		family := "site"
		if strings.HasPrefix(c.Prefix, "/usr") {
			family = "system"
		}
		out = append(out, model.CompilerProvider{
			Name: c.Name, Version: c.Version, Prefix: c.Prefix,
			ProviderFamily: family, Languages: c.Languages, Modules: c.Modules,
		})
	}
	return out
}

// toMPIProviders transforms the probe-detected cray-mpich + generic MPI
// externals into the generic mpi_providers inventory.
func toMPIProviders(f model.SystemFragment) []model.MPIProvider {
	out := []model.MPIProvider{}
	if f.VendorCray != nil && f.VendorCray.CrayMPICH != nil {
		cm := f.VendorCray.CrayMPICH
		flavors := map[string]model.MPIFlavor{}
		compilers := make([]string, 0, len(cm.Flavors))
		for name, fl := range cm.Flavors {
			flavors[name] = model.MPIFlavor{Prefix: fl.Prefix, Modules: fl.Modules}
			compilers = append(compilers, name)
		}
		sort.Strings(compilers)
		out = append(out, model.MPIProvider{
			Name: "cray-mpich", Version: cm.Version, ProviderFamily: "cray-pe",
			Compatibility: &model.MPICompatibility{Compilers: compilers},
			Flavors:       flavors,
		})
	}
	for _, m := range f.MPI {
		out = append(out, model.MPIProvider{
			Name: m.Name, Version: m.Version, ProviderFamily: "site",
			Prefix: m.Prefix, Compiler: m.Compiler, Modules: m.Modules,
		})
	}
	return out
}

// NewMergeCommand returns the `cluster-inspector merge` subcommand.
//
// Deterministically combines a system fragment plus one or more node
// fragments into a single profile.yaml.
func NewMergeCommand() *cobra.Command {
	var systemFragmentPath string
	var nodeFragmentPaths []string
	var outputPath string

	cmd := &cobra.Command{
		Use:   "merge",
		Short: "Merge system + node fragments into one profile.yaml",
		Long: `merge consolidates a system fragment and one or more node
fragments into one valid profile.yaml. The output is deterministic: the
same inputs always produce byte-identical YAML.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, err := readAndMergeFragments(systemFragmentPath, nodeFragmentPaths)
			if err != nil {
				return err
			}
			if err := model.ValidateProfile(profile); err != nil {
				return err
			}
			return writeProfileOutput(cmd, outputPath, profile)
		},
	}
	cmd.Flags().StringVar(&systemFragmentPath, "system-fragment", "", "path to system fragment YAML")
	cmd.Flags().StringArrayVar(&nodeFragmentPaths, "node", nil, "path to node fragment YAML; may be repeated")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "write merged profile.yaml to this path instead of stdout")
	return cmd
}

func readAndMergeFragments(systemFragmentPath string, nodeFragmentPaths []string) (*model.Profile, error) {
	if systemFragmentPath == "" {
		return nil, fmt.Errorf("merge requires --system-fragment")
	}
	if len(nodeFragmentPaths) == 0 {
		return nil, fmt.Errorf("merge requires at least one --node")
	}
	systemFragment, err := readSystemFragment(systemFragmentPath)
	if err != nil {
		return nil, err
	}
	nodeFragments := make([]model.NodeFragment, 0, len(nodeFragmentPaths))
	for _, path := range nodeFragmentPaths {
		nodeFragment, err := readNodeFragment(path)
		if err != nil {
			return nil, err
		}
		nodeFragments = append(nodeFragments, nodeFragment)
	}
	return mergeFragments(systemFragment, nodeFragments)
}

func mergeFragments(systemFragment model.SystemFragment, nodeFragments []model.NodeFragment) (*model.Profile, error) {
	schemaVersion := systemFragment.SchemaVersion
	if schemaVersion == 0 {
		schemaVersion = 1
	}
	nodeTypes := make(map[string]model.NodeType, len(nodeFragments))
	for _, fragment := range nodeFragments {
		if fragment.Name == "" {
			return nil, fmt.Errorf("node fragment is missing name")
		}
		if _, exists := nodeTypes[fragment.Name]; exists {
			return nil, fmt.Errorf("duplicate node fragment %q", fragment.Name)
		}
		if err := validateNodeFragmentArgs(fragment.Name, fragment.Role); err != nil {
			return nil, err
		}
		nodeTypes[fragment.Name] = model.NodeType{
			Role:        fragment.Role,
			Description: fragment.Description,
			CPU:         fragment.CPU,
			GPU:         fragment.GPU,
			BuildStage:  fragment.BuildStage,
		}
	}

	return &model.Profile{
		SchemaVersion:     schemaVersion,
		System:            systemFragment.System,
		OS:                systemFragment.OS,
		Fabric:            systemFragment.Fabric,
		ModulesSystem:     systemFragment.ModulesSystem,
		CompilerProviders: toCompilerProviders(systemFragment),
		MPIProviders:      toMPIProviders(systemFragment),
		GPUToolkitModules: systemFragment.GPUToolkitModules,
		Filesystem:        systemFragment.Filesystem,
		NodeTypes:         nodeTypes,
	}, nil
}

func readSystemFragment(path string) (model.SystemFragment, error) {
	var fragment model.SystemFragment
	if err := readYAMLFile(path, &fragment); err != nil {
		return model.SystemFragment{}, fmt.Errorf("read system fragment %q: %w", path, err)
	}
	return fragment, nil
}

func readNodeFragment(path string) (model.NodeFragment, error) {
	var fragment model.NodeFragment
	if err := readYAMLFile(path, &fragment); err != nil {
		return model.NodeFragment{}, fmt.Errorf("read node fragment %q: %w", path, err)
	}
	return fragment, nil
}

func readYAMLFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return err
	}
	return nil
}
