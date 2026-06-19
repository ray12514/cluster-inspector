package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ray12514/cluster-inspector/internal/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewVerifyCommand returns the `cluster-inspector verify` subcommand.
//
// Validates a profile.yaml — whether produced by this tool or written by
// hand — against the canonical schema and semantic rules.
// TODO: Phase 5 — add full semantic rule set per the design doc.
func NewVerifyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify [profile.yaml]",
		Short: "Validate a profile.yaml against the schema and semantic rules",
		Long: `verify validates a profile.yaml against:
  - the canonical JSON Schema (profile-v1.json)
  - semantic rules (at least one build_host, GPU lanes only when GPU node
    types exist, lanes_capable references valid node types, etc.)

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

	return writeVerifyLine(cmd, "PASS schema")
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
