package commands

import (
	"fmt"
	"os"

	inspectorhints "github.com/ray12514/cluster-inspector/internal/hints"
	"github.com/ray12514/cluster-inspector/internal/model"
	"github.com/ray12514/cluster-inspector/internal/output"
	"github.com/ray12514/cluster-inspector/internal/probes"
	"github.com/spf13/cobra"
)

// NewProbeSystemCommand returns the `cluster-inspector probe-system`
// subcommand.
//
// Runs the system-wide probes (OS, glibc, modules, fabric, vendor_cray,
// compilers_external, mpi, gpu_toolkit_modules, system_externals, filesystem) and emits a
// system fragment.
func NewProbeSystemCommand() *cobra.Command {
	var systemName string
	var hintsPath string
	var outputPath string

	cmd := &cobra.Command{
		Use:   "probe-system",
		Short: "Probe system-wide facts and emit a system fragment",
		Long: `probe-system collects system-wide facts (OS, glibc, module tool,
fabric, Cray PE inventory, compiler externals, MPI externals, GPU toolkit
modules, system package externals, install-tree candidates) and emits a system fragment that can be
merged with per-node fragments by the merge command.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			hints, err := loadHints(hintsPath)
			if err != nil {
				return err
			}
			fragment := buildSystemFragment(systemName, hints)
			return writeSystemFragmentOutput(cmd, outputPath, fragment)
		},
	}
	cmd.Flags().StringVar(&systemName, "system", "", "system name for emitted fragment")
	cmd.Flags().StringVar(&hintsPath, "hints", "", "path to inspector-hints.yaml")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "write system fragment to this path instead of stdout")
	return cmd
}

func buildSystemFragment(systemName string, hints *inspectorhints.Hints) *model.SystemFragment {
	system := probes.ProbeSystem(systemName)
	modules := probes.ProbeModules()
	fabric := probes.ProbeFabric()
	cray := probes.ProbeCrayPEWithModules(modules.Candidates, hints)
	compilers := probes.ProbeCompilersExternalWithModules(modules.Candidates, hints)
	mpi := probes.ProbeMPIWithModules(modules.Candidates, hints)
	gpuToolkits := probes.ProbeGPUToolkitModulesWithModules(modules.Candidates, hints)
	systemExternals := probes.ProbeSystemExternals()
	filesystem := probes.ProbeFilesystem()

	evidence := map[string]model.Evidence{}
	mergeEvidence(evidence, system.Evidence)
	mergeEvidence(evidence, modules.Evidence)
	mergeEvidence(evidence, fabric.Evidence)
	mergeEvidence(evidence, cray.Evidence)
	mergeEvidence(evidence, compilers.Evidence)
	mergeEvidence(evidence, mpi.Evidence)
	mergeEvidence(evidence, gpuToolkits.Evidence)
	mergeEvidence(evidence, systemExternals.Evidence)
	mergeEvidence(evidence, filesystem.Evidence)

	return &model.SystemFragment{
		SchemaVersion:     1,
		System:            system.System,
		OS:                system.OS,
		Fabric:            fabric.Fabric,
		ModulesSystem:     modules.ModulesSystem,
		VendorCray:        cray.VendorCray,
		CompilersExternal: compilers.Compilers,
		MPI:               mpi.MPI,
		GPUToolkitModules: gpuToolkits.Toolkits,
		SystemExternals:   systemExternals.Externals,
		Filesystem:        filesystem.Filesystem,
		ModulePaths:       modules.ModulePaths,
		Evidence:          evidence,
	}
}

func loadHints(path string) (*inspectorhints.Hints, error) {
	if path == "" {
		return nil, nil
	}
	return inspectorhints.LoadFile(path)
}

func mergeEvidence(dst map[string]model.Evidence, src map[string]model.Evidence) {
	for key, value := range src {
		dst[key] = value
	}
}

func writeSystemFragmentOutput(cmd *cobra.Command, outputPath string, fragment *model.SystemFragment) error {
	if outputPath == "" {
		return output.WriteSystemFragment(cmd.OutOrStdout(), fragment)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("open output %q: %w", outputPath, err)
	}
	if err := output.WriteSystemFragment(f, fragment); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
