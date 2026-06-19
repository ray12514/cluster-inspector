package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ray12514/cluster-inspector/internal/model"
	"github.com/ray12514/cluster-inspector/internal/output"
	"github.com/spf13/cobra"
)

// NewProfileCommand returns the `cluster-inspector profile` subcommand.
//
// All-in-one: orchestrates probe-system + per-node probe-node + merge into
// a single profile.yaml. The common operator entry point.
// Phase 1 emits a minimal local skeleton for this: node-type specs. Later
// phases replace the placeholder facts with real probes.
// See stack-planning/docs/cluster_inspector_stack_profile_design_v1.md
// § CLI Contract for the canonical argument shape.
func NewProfileCommand() *cobra.Command {
	var systemName string
	var hintsPath string
	var nodeTypeSpecs []string
	var outputPath string

	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Probe system + node types and emit a merged profile.yaml",
		Long: `profile is the all-in-one entry point. It runs probe-system
on the current host, runs one probe-node per --node-type spec (in-shell,
through srun, or through pbsdsh), and merges the fragments into a single
profile.yaml.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = hintsPath
			profile, err := buildLocalSkeletonProfile(systemName, nodeTypeSpecs)
			if err != nil {
				return err
			}
			if err := model.ValidateProfile(profile); err != nil {
				return err
			}
			return writeProfileOutput(cmd, outputPath, profile)
		},
	}
	cmd.Flags().StringVar(&systemName, "system", "", "system name for profile.yaml")
	cmd.Flags().StringVar(&hintsPath, "hints", "", "path to inspector-hints.yaml")
	cmd.Flags().StringArrayVar(&nodeTypeSpecs, "node-type", nil, "node-type spec such as login=this:role=both")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "write profile.yaml to this path instead of stdout")
	return cmd
}

func buildLocalSkeletonProfile(systemName string, nodeTypeSpecs []string) (*model.Profile, error) {
	if systemName == "" {
		return nil, errors.New("profile requires --system")
	}
	if len(nodeTypeSpecs) == 0 {
		return nil, errors.New("profile requires at least one --node-type")
	}

	nodeTypes := make(map[string]model.NodeType, len(nodeTypeSpecs))
	for _, spec := range nodeTypeSpecs {
		name, role, err := parsePhase1NodeTypeSpec(spec)
		if err != nil {
			return nil, err
		}
		if _, exists := nodeTypes[name]; exists {
			return nil, fmt.Errorf("duplicate --node-type %q", name)
		}
		nodeTypes[name] = model.NodeType{
			Role:        role,
			Description: "Minimal local skeleton",
			CPU: model.CPU{
				Detected:  "x86_64",
				Preferred: "x86_64",
			},
			GPU: nil,
			BuildStage: []model.BuildStage{{
				Path:            "/tmp/cluster-inspector/spack-stage",
				Visibility:      "node-local",
				Writable:        true,
				ThroughputClass: "unknown",
			}},
		}
	}

	return &model.Profile{
		SchemaVersion: 1,
		System: model.System{
			Name:        systemName,
			Family:      "linux-local",
			Description: "Minimal homogeneous local skeleton",
		},
		OS: model.OS{
			Name:  "linux",
			Major: 0,
			Glibc: "0.0",
		},
		Fabric: model.Fabric{
			Type:    "ethernet",
			Drivers: []model.NamedPrefixVersioned{},
		},
		ModulesSystem: model.ModulesSystem{
			Tool: "lmod",
		},
		VendorCray: nil,
		Filesystem: model.Filesystem{
			InstallTreeCandidates: []model.InstallTreeCandidate{{
				Path:         "/tmp/cluster-inspector/install-tree",
				Type:         "local",
				LocksHonored: true,
			}},
		},
		NodeTypes: nodeTypes,
	}, nil
}

func parsePhase1NodeTypeSpec(spec string) (string, string, error) {
	name, rest, ok := strings.Cut(spec, "=")
	if !ok || name == "" || rest == "" {
		return "", "", fmt.Errorf("invalid --node-type %q: want name=this:role=<build_host|runtime|both>", spec)
	}

	parts := strings.Split(rest, ":")
	if parts[0] != "this" {
		return "", "", fmt.Errorf("phase 1 profile skeleton supports only this: node-type specs, got %q", spec)
	}

	role := ""
	for _, part := range parts[1:] {
		key, value, ok := strings.Cut(part, "=")
		if ok && key == "role" {
			role = value
		}
	}
	if role != "build_host" && role != "runtime" && role != "both" {
		return "", "", fmt.Errorf("invalid role in --node-type %q: want build_host, runtime, or both", spec)
	}
	return name, role, nil
}

func writeProfileOutput(cmd *cobra.Command, outputPath string, profile *model.Profile) error {
	if outputPath == "" {
		return output.WriteProfile(cmd.OutOrStdout(), profile)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("open output %q: %w", outputPath, err)
	}

	if err := output.WriteProfile(f, profile); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
