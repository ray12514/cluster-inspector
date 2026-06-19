package commands

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/ray12514/cluster-inspector/internal/model"
	"github.com/ray12514/cluster-inspector/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewProfileCommand returns the `cluster-inspector profile` subcommand.
//
// All-in-one: orchestrates probe-system + per-node probe-node + merge into
// a single profile.yaml. The common operator entry point.
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
			profile, err := buildProbedProfile(systemName, nodeTypeSpecs)
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

type profileNodeTypeSpec struct {
	Name        string
	Role        string
	Description string
	Runner      nodeRunner
}

func buildProbedProfile(systemName string, nodeTypeSpecs []string) (*model.Profile, error) {
	if systemName == "" {
		return nil, fmt.Errorf("profile requires --system")
	}
	if len(nodeTypeSpecs) == 0 {
		return nil, fmt.Errorf("profile requires at least one --node-type")
	}

	parsedSpecs, err := parseProfileNodeTypeSpecs(nodeTypeSpecs)
	if err != nil {
		return nil, err
	}
	systemFragment := buildSystemFragment(systemName)
	nodeFragments := make([]model.NodeFragment, 0, len(parsedSpecs))
	for _, spec := range parsedSpecs {
		fragment, err := probeProfileNodeType(spec)
		if err != nil {
			return nil, err
		}
		nodeFragments = append(nodeFragments, fragment)
	}
	return mergeFragments(*systemFragment, nodeFragments)
}

func probeProfileNodeType(spec profileNodeTypeSpec) (model.NodeFragment, error) {
	if spec.Runner.Kind == "this" {
		return *buildNodeFragment(spec.Name, spec.Role, spec.Description), nil
	}
	raw, err := remoteProbeNodeOutput(spec.Runner, spec.Name, spec.Role, spec.Description)
	if err != nil {
		return model.NodeFragment{}, err
	}
	var fragment model.NodeFragment
	if err := yamlDecode(raw, &fragment); err != nil {
		return model.NodeFragment{}, fmt.Errorf("parse %s node fragment %q: %w", spec.Runner.Kind, spec.Name, err)
	}
	return fragment, nil
}

func parseProfileNodeTypeSpecs(specs []string) ([]profileNodeTypeSpec, error) {
	parsed := make([]profileNodeTypeSpec, 0, len(specs))
	seen := map[string]bool{}
	for _, spec := range specs {
		nodeType, err := parseProfileNodeTypeSpec(spec)
		if err != nil {
			return nil, err
		}
		if seen[nodeType.Name] {
			return nil, fmt.Errorf("duplicate --node-type %q", nodeType.Name)
		}
		seen[nodeType.Name] = true
		parsed = append(parsed, nodeType)
	}
	return parsed, nil
}

func parseProfileNodeTypeSpec(spec string) (profileNodeTypeSpec, error) {
	name, rest, ok := strings.Cut(spec, "=")
	if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(rest) == "" {
		return profileNodeTypeSpec{}, fmt.Errorf("invalid --node-type %q: want name=<this|srun|pbsdsh>:role=<build_host|runtime|both>", spec)
	}
	name = strings.TrimSpace(name)

	parts := strings.Split(rest, ":")
	runnerKind := strings.TrimSpace(parts[0])
	role := ""
	description := ""
	runnerSpecParts := make([]string, 0, len(parts)-1)
	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if ok {
			switch strings.TrimSpace(key) {
			case "role":
				role = strings.TrimSpace(value)
				continue
			case "description":
				description = strings.TrimSpace(value)
				continue
			}
		}
		runnerSpecParts = append(runnerSpecParts, part)
	}
	if role == "" {
		return profileNodeTypeSpec{}, fmt.Errorf("invalid --node-type %q: missing role=<build_host|runtime|both>", spec)
	}
	runnerSpec := runnerKind
	if len(runnerSpecParts) > 0 {
		runnerSpec += ":" + strings.Join(runnerSpecParts, ":")
	}
	runner, err := parseNodeRunnerSpec(runnerSpec, nil)
	if err != nil {
		return profileNodeTypeSpec{}, err
	}
	if err := validateNodeFragmentArgs(name, role); err != nil {
		return profileNodeTypeSpec{}, err
	}
	return profileNodeTypeSpec{Name: name, Role: role, Description: description, Runner: runner}, nil
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

func yamlDecode(data []byte, out any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	return decoder.Decode(out)
}
