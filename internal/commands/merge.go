package commands

import (
	"fmt"
	"os"

	"github.com/ray12514/cluster-inspector/internal/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

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
		VendorCray:        systemFragment.VendorCray,
		CompilersExternal: systemFragment.CompilersExternal,
		MPI:               systemFragment.MPI,
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
