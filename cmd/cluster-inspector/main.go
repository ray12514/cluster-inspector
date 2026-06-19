// Command cluster-inspector probes a system and emits a profile.yaml
// matching the canonical schema in stack-planning/schemas/profile-v1.json.
//
// See AGENTS.md for the implementation queue and conventions.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ray12514/cluster-inspector/internal/commands"
)

// Version is set at build time via -ldflags "-X main.Version=..."
var Version = "0.0.0-dev"

func main() {
	root := &cobra.Command{
		Use:   "cluster-inspector",
		Short: "Probe a system and emit a Spack stack profile.yaml",
		Long: `cluster-inspector probes a host (and optionally per-node-type) and
produces a profile.yaml matching the canonical schema in stack-planning.

The tool is read-only on every host it touches. It does not call Spack,
does not render anything, and does not deploy. Operator hints in
inspector-hints.yaml are the committed override mechanism for module
discovery.

See the design doc in stack-planning for product boundary and per-field
extraction rules.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		commands.NewProfileCommand(),
		commands.NewProbeSystemCommand(),
		commands.NewProbeNodeCommand(),
		commands.NewMergeCommand(),
		commands.NewVerifyCommand(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "cluster-inspector: %v\n", err)
		os.Exit(1)
	}
}
