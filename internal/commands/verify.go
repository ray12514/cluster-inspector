package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ray12514/cluster-inspector/internal/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewVerifyCommand returns the `cluster-inspector verify` subcommand.
//
// Validates a profile.yaml — whether produced by this tool or written by
// hand — against the canonical schema and semantic rules.
func NewVerifyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify [profile.yaml]",
		Short: "Validate a profile.yaml against the schema and semantic rules",
		Long: `verify validates a profile.yaml against:
  - the canonical JSON Schema (profile-v1.json)
  - semantic rules (build/runtime node coverage, GPU toolkit consistency,
    Cray MPICH flavor consistency, and site MPI compiler references)

Pass exits 0; fail exits non-zero with a list of failing rules.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return verifyProfile(cmd, args[0])
		},
	}
	return cmd
}

func verifyProfile(cmd *cobra.Command, profilePath string) error {
	schema, err := model.CompileProfileSchema()
	if err != nil {
		return err
	}

	profile, err := readProfileYAML(profilePath)
	if err != nil {
		if printErr := writeVerifyLine(cmd, "FAIL schema"); printErr != nil {
			return printErr
		}
		return err
	}

	if err := schema.Validate(profile); err != nil {
		if printErr := writeVerifyLine(cmd, "FAIL schema"); printErr != nil {
			return printErr
		}
		return formatSchemaError(err)
	}

	if err := writeVerifyLine(cmd, "PASS schema"); err != nil {
		return err
	}
	typedProfile, err := readProfileModel(profilePath)
	if err != nil {
		if printErr := writeVerifyLine(cmd, "FAIL semantic"); printErr != nil {
			return printErr
		}
		return err
	}
	semanticErrors := validateProfileSemantics(typedProfile)
	if len(semanticErrors) > 0 {
		if printErr := writeVerifyLine(cmd, "FAIL semantic"); printErr != nil {
			return printErr
		}
		return fmt.Errorf("profile failed semantic verification: %s", strings.Join(semanticErrors, "; "))
	}
	return writeVerifyLine(cmd, "PASS semantic")
}

func writeVerifyLine(cmd *cobra.Command, line string) error {
	_, err := fmt.Fprintln(cmd.OutOrStdout(), line)
	return err
}

func readProfileYAML(profilePath string) (any, error) {
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("read profile %q: %w", profilePath, err)
	}

	var profile any
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("parse profile %q: %w", profilePath, err)
	}

	profile = jsonCompatible(profile)
	jsonData, err := json.Marshal(profile)
	if err != nil {
		return nil, fmt.Errorf("normalize profile %q: %w", profilePath, err)
	}
	var normalized any
	if err := json.Unmarshal(jsonData, &normalized); err != nil {
		return nil, fmt.Errorf("normalize profile %q: %w", profilePath, err)
	}
	return normalized, nil
}

func jsonCompatible(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = jsonCompatible(v)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[fmt.Sprint(k)] = jsonCompatible(v)
		}
		return out
	case []any:
		for i, v := range x {
			x[i] = jsonCompatible(v)
		}
		return x
	default:
		return x
	}
}

func formatSchemaError(err error) error {
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return fmt.Errorf("profile does not match embedded profile schema")
	}
	return fmt.Errorf("profile does not match embedded profile schema: %s", msg)
}

func readProfileModel(profilePath string) (*model.Profile, error) {
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("read profile %q: %w", profilePath, err)
	}
	var profile model.Profile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("parse profile %q: %w", profilePath, err)
	}
	return &profile, nil
}

func validateProfileSemantics(profile *model.Profile) []string {
	if profile == nil {
		return []string{"profile is nil"}
	}
	errors := []string{}
	errors = append(errors, validateNodeRoleSemantics(profile)...)
	errors = append(errors, validateGPUToolkitSemantics(profile)...)
	errors = append(errors, validateCraySemantics(profile)...)
	errors = append(errors, validateMPISemantics(profile)...)
	errors = append(errors, validateFabricSemantics(profile)...)
	return errors
}

func validateNodeRoleSemantics(profile *model.Profile) []string {
	hasBuildHost := false
	hasRuntime := false
	for _, node := range profile.NodeTypes {
		if node.Role == "build_host" || node.Role == "both" {
			hasBuildHost = true
		}
		if node.Role == "runtime" || node.Role == "both" {
			hasRuntime = true
		}
	}
	errors := []string{}
	if !hasBuildHost {
		errors = append(errors, "node_types must include at least one build_host or both node")
	}
	if !hasRuntime {
		errors = append(errors, "node_types must include at least one runtime or both node")
	}
	return errors
}

func validateGPUToolkitSemantics(profile *model.Profile) []string {
	errors := []string{}
	for name, node := range profile.NodeTypes {
		if node.GPU == nil {
			continue
		}
		switch node.GPU.Vendor {
		case "amd":
			if profile.GPUToolkitModules == nil || profile.GPUToolkitModules.ROCm == nil {
				errors = append(errors, fmt.Sprintf("node_types.%s has AMD GPU but gpu_toolkit_modules.rocm is absent", name))
			}
		case "nvidia":
			if profile.GPUToolkitModules == nil || (profile.GPUToolkitModules.CUDAToolkit == nil && profile.GPUToolkitModules.NVHPC == nil) {
				errors = append(errors, fmt.Sprintf("node_types.%s has NVIDIA GPU but no CUDA/NVHPC toolkit is present", name))
			}
		}
	}
	if profile.GPUToolkitModules != nil && profile.GPUToolkitModules.ROCm != nil {
		errors = append(errors, validateROCmComponents(profile.GPUToolkitModules.ROCm)...)
	}
	return errors
}

func validateROCmComponents(rocm *model.ROCmToolkitModule) []string {
	if rocm == nil {
		return nil
	}
	required := map[string]bool{
		"hip":          false,
		"hsa-rocr-dev": false,
		"llvm-amdgpu":  false,
	}
	errors := []string{}
	for _, component := range rocm.SpackComponents {
		if _, ok := required[component.Package]; ok {
			required[component.Package] = true
		}
		if component.Prefix == "" || !filepath.IsAbs(component.Prefix) {
			errors = append(errors, fmt.Sprintf("gpu_toolkit_modules.rocm component %q must have an absolute prefix", component.Package))
		}
	}
	for pkg, present := range required {
		if !present {
			errors = append(errors, "gpu_toolkit_modules.rocm missing required component "+pkg)
		}
	}
	return errors
}

func validateCraySemantics(profile *model.Profile) []string {
	if profile.VendorCray == nil || profile.VendorCray.CrayMPICH == nil {
		return nil
	}
	errors := []string{}
	for flavor := range profile.VendorCray.CrayMPICH.Flavors {
		switch flavor {
		case "cce", "gcc", "aocc", "intel", "rocmcc", "nvhpc":
		default:
			errors = append(errors, "vendor_cray.cray_mpich.flavors contains unsupported flavor "+flavor)
		}
	}
	return errors
}

func validateMPISemantics(profile *model.Profile) []string {
	errors := []string{}
	for i, mpi := range profile.MPI {
		if strings.TrimSpace(mpi.Compiler) == "" {
			continue
		}
		if !compilerReferenceExists(profile.CompilersExternal, mpi.Compiler) {
			errors = append(errors, fmt.Sprintf("mpi[%d].compiler %q does not match compilers_external", i, mpi.Compiler))
		}
	}
	return errors
}

func compilerReferenceExists(compilers []model.CompilerExternal, ref string) bool {
	name, version, _ := strings.Cut(ref, "@")
	for _, compiler := range compilers {
		if compiler.Name != name {
			continue
		}
		if version == "" || compiler.Version == version || strings.HasPrefix(compiler.Version, version) || strings.HasPrefix(version, compiler.Version) {
			return true
		}
	}
	return false
}

func validateFabricSemantics(profile *model.Profile) []string {
	if profile.Fabric.Type != "ethernet" && len(profile.Fabric.Drivers) == 0 {
		return []string{"non-ethernet fabric must include at least one fabric driver"}
	}
	return nil
}
