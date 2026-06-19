package commands

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ray12514/cluster-inspector/internal/model"
	"github.com/ray12514/cluster-inspector/internal/output"
	"github.com/ray12514/cluster-inspector/internal/probes"
	"github.com/spf13/cobra"
)

type nodeRunner struct {
	Kind string
	Args []string
}

// NewProbeNodeCommand returns the `cluster-inspector probe-node`
// subcommand.
//
// Runs per-node-type probes (CPU target, GPU facts, build-stage
// candidates) and emits a node fragment.
func NewProbeNodeCommand() *cobra.Command {
	var nodeType string
	var role string
	var description string
	var runnerSpec string
	var runnerArgs []string
	var outputPath string

	cmd := &cobra.Command{
		Use:   "probe-node",
		Short: "Probe a node-type's CPU/GPU/build-stage facts",
		Long: `probe-node runs on a single node class and emits a node fragment
covering CPU target, GPU presence and arch, and writable build-stage
candidates. Fragments are merged into the full profile.yaml by the merge
command.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateNodeFragmentArgs(nodeType, role); err != nil {
				return err
			}
			runner, err := parseNodeRunnerSpec(runnerSpec, runnerArgs)
			if err != nil {
				return err
			}
			if runner.Kind != "this" {
				return runRemoteProbeNode(cmd, runner, nodeType, role, description, outputPath)
			}
			fragment := buildNodeFragment(nodeType, role, description)
			return writeNodeFragmentOutput(cmd, outputPath, fragment)
		},
	}
	cmd.Flags().StringVar(&nodeType, "node-type", "", "node type name for emitted fragment")
	cmd.Flags().StringVar(&role, "role", "", "node role: build_host, runtime, or both")
	cmd.Flags().StringVar(&description, "description", "", "optional node type description")
	cmd.Flags().StringVar(&runnerSpec, "runner", "this", "node runner: this, srun[:key=value,...], or pbsdsh[:key=value,...]")
	cmd.Flags().StringArrayVar(&runnerArgs, "runner-arg", nil, "raw argument passed to srun/pbsdsh before the probe command; may be repeated")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "write node fragment to this path instead of stdout")
	return cmd
}

func validateNodeFragmentArgs(nodeType, role string) error {
	if nodeType == "" {
		return fmt.Errorf("probe-node requires --node-type")
	}
	if role != "build_host" && role != "runtime" && role != "both" {
		return fmt.Errorf("invalid --role %q: want build_host, runtime, or both", role)
	}
	return nil
}

func buildNodeFragment(nodeType, role, description string) *model.NodeFragment {
	node := probes.ProbeNode()
	return &model.NodeFragment{
		Name:        nodeType,
		Role:        role,
		Description: description,
		CPU:         node.CPU,
		GPU:         node.GPU,
		BuildStage:  node.BuildStage,
		Evidence:    node.Evidence,
	}
}

func writeNodeFragmentOutput(cmd *cobra.Command, outputPath string, fragment *model.NodeFragment) error {
	if outputPath == "" {
		return output.WriteNodeFragment(cmd.OutOrStdout(), fragment)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("open output %q: %w", outputPath, err)
	}
	if err := output.WriteNodeFragment(f, fragment); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func parseNodeRunnerSpec(spec string, extraArgs []string) (nodeRunner, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		spec = "this"
	}
	kind, rest, hasRest := strings.Cut(spec, ":")
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return nodeRunner{}, fmt.Errorf("invalid --runner %q", spec)
	}
	runner := nodeRunner{Kind: kind}
	if hasRest && strings.TrimSpace(rest) != "" {
		runner.Args = append(runner.Args, runnerArgsFromSpec(kind, rest)...)
	}
	runner.Args = append(runner.Args, extraArgs...)
	switch runner.Kind {
	case "this":
		if len(runner.Args) > 0 {
			return nodeRunner{}, fmt.Errorf("runner this does not accept runner arguments")
		}
	case "srun", "pbsdsh":
	default:
		return nodeRunner{}, fmt.Errorf("invalid --runner %q: want this, srun, or pbsdsh", spec)
	}
	return runner, nil
}

func runnerArgsFromSpec(kind, spec string) []string {
	fields := strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == ':' })
	args := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			args = append(args, field)
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if kind == "srun" {
			args = append(args, "--"+key+"="+value)
		} else {
			args = append(args, "-"+key, value)
		}
	}
	return args
}

func runRemoteProbeNode(cmd *cobra.Command, runner nodeRunner, nodeType, role, description, outputPath string) error {
	stdout, err := remoteProbeNodeOutput(runner, nodeType, role, description)
	if err != nil {
		return err
	}

	if outputPath == "" {
		_, err := cmd.OutOrStdout().Write(stdout)
		return err
	}
	return os.WriteFile(outputPath, stdout, 0o644)
}

func remoteProbeNodeOutput(runner nodeRunner, nodeType, role, description string) ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve current executable: %w", err)
	}

	probeArgs := []string{exe, "probe-node", "--node-type", nodeType, "--role", role, "--runner", "this"}
	if description != "" {
		probeArgs = append(probeArgs, "--description", description)
	}
	runnerArgs := append([]string{}, runner.Args...)
	runnerArgs = append(runnerArgs, probeArgs...)

	remote := exec.Command(runner.Kind, runnerArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	remote.Stdout = &stdout
	remote.Stderr = &stderr
	if err := remote.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%s probe-node failed: %w: %s", runner.Kind, err, msg)
		}
		return nil, fmt.Errorf("%s probe-node failed: %w", runner.Kind, err)
	}
	return stdout.Bytes(), nil
}
