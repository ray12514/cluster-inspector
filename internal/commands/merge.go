package commands

import (
	"errors"

	"github.com/spf13/cobra"
)

// NewMergeCommand returns the `cluster-inspector merge` subcommand.
//
// Deterministically combines a system fragment plus one or more node
// fragments into a single profile.yaml.
// TODO: Phase 3 — implement deterministic merge with stable key order;
// re-running on the same inputs must produce byte-identical output.
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
			_, _, _ = systemFragmentPath, nodeFragmentPaths, outputPath
			return errors.New("cluster-inspector merge: not yet implemented (Phase 3 merge)")
		},
	}
	cmd.Flags().StringVar(&systemFragmentPath, "system-fragment", "", "path to system fragment YAML")
	cmd.Flags().StringArrayVar(&nodeFragmentPaths, "node", nil, "path to node fragment YAML; may be repeated")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "write merged profile.yaml to this path instead of stdout")
	return cmd
}
