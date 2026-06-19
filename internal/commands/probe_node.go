package commands

import (
	"errors"

	"github.com/spf13/cobra"
)

// NewProbeNodeCommand returns the `cluster-inspector probe-node`
// subcommand.
//
// Runs per-node-type probes (CPU target, GPU facts, build-stage
// candidates) and emits a node fragment.
// TODO: Phase 3 — call internal/probes/node and build the node fragment.
func NewProbeNodeCommand() *cobra.Command {
	var nodeType string
	var role string
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
			_, _, _ = nodeType, role, outputPath
			return errors.New("cluster-inspector probe-node: not yet implemented (Phase 3 probes)")
		},
	}
	cmd.Flags().StringVar(&nodeType, "node-type", "", "node type name for emitted fragment")
	cmd.Flags().StringVar(&role, "role", "", "node role: build_host, runtime, or both")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "write node fragment to this path instead of stdout")
	return cmd
}
