// Package hints loads and validates inspector-hints.yaml, the committed
// override mechanism for module discovery.
//
// See stack-planning/docs/cluster_inspector_stack_profile_design_v1.md
// § Module Discovery And Hints for the full schema; the example hints
// file under that section is the reference.
package hints

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const schemaVersion = 1

// Hints mirrors inspector-hints.yaml.
type Hints struct {
	SchemaVersion   int            `yaml:"schema_version"`
	Compilers       ModuleHints    `yaml:"compilers"`
	MPI             ModuleHints    `yaml:"mpi"`
	GPUToolkits     ModuleHints    `yaml:"gpu_toolkits"`
	FabricUserspace ModuleHints    `yaml:"fabric_userspace"`
	SystemExternals ModuleHints    `yaml:"system_externals"`
	Extras          ExplicitExtras `yaml:"extras"`
}

type ModuleHints struct {
	Include         []string `yaml:"include"`
	ExcludePatterns []string `yaml:"exclude_patterns"`
}

type ExplicitExtras struct {
	Compilers       []CompilerExtra        `yaml:"compilers"`
	MPI             []MPIExtra             `yaml:"mpi"`
	GPUToolkits     []GPUToolkitExtra      `yaml:"gpu_toolkits"`
	FabricUserspace []FabricUserspaceExtra `yaml:"fabric_userspace"`
}

type CompilerExtra struct {
	Module    string   `yaml:"module"`
	Name      string   `yaml:"name"`
	Version   string   `yaml:"version"`
	Prefix    string   `yaml:"prefix"`
	Languages []string `yaml:"languages"`
}

type MPIExtra struct {
	Module     string `yaml:"module"`
	Name       string `yaml:"name"`
	Provenance string `yaml:"provenance"`
	Version    string `yaml:"version"`
	Prefix     string `yaml:"prefix"`
	Compiler   string `yaml:"compiler"`
}

type GPUToolkitExtra struct {
	Module          string           `yaml:"module"`
	Name            string           `yaml:"name"`
	Version         string           `yaml:"version"`
	Prefix          string           `yaml:"prefix"`
	SpackComponents []SpackComponent `yaml:"spack_components"`
}

type SpackComponent struct {
	Package string `yaml:"package"`
	Prefix  string `yaml:"prefix"`
}

type FabricUserspaceExtra struct {
	Module  string `yaml:"module"`
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Prefix  string `yaml:"prefix"`
}

// LoadFile reads and validates an inspector-hints.yaml file from an
// operator-provided path.
func LoadFile(filePath string) (*Hints, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open hints %q: %w", filePath, err)
	}
	defer func() {
		_ = file.Close()
	}()
	hints, err := Parse(file)
	if err != nil {
		return nil, fmt.Errorf("parse hints %q: %w", filePath, err)
	}
	return hints, nil
}

// Parse decodes and validates inspector-hints.yaml. Unknown YAML fields
// are rejected so committed hints do not silently drift from the contract.
func Parse(r io.Reader) (*Hints, error) {
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)
	var hints Hints
	if err := decoder.Decode(&hints); err != nil {
		return nil, err
	}
	if err := hints.Validate(); err != nil {
		return nil, err
	}
	return &hints, nil
}

// Validate checks the shape that is not expressible through the Go YAML
// structs alone: schema version, valid glob patterns, required extra
// fields, absolute prefixes, and allowed enum-like values.
func (h *Hints) Validate() error {
	if h == nil {
		return fmt.Errorf("hints: nil document")
	}
	if h.SchemaVersion != schemaVersion {
		return fmt.Errorf("hints.schema_version = %d, want %d", h.SchemaVersion, schemaVersion)
	}
	if err := validateModuleHints("compilers", h.Compilers); err != nil {
		return err
	}
	if err := validateModuleHints("mpi", h.MPI); err != nil {
		return err
	}
	if err := validateModuleHints("gpu_toolkits", h.GPUToolkits); err != nil {
		return err
	}
	if err := validateModuleHints("fabric_userspace", h.FabricUserspace); err != nil {
		return err
	}
	if err := validateModuleHints("system_externals", h.SystemExternals); err != nil {
		return err
	}
	return validateExtras(h.Extras)
}

func validateModuleHints(section string, hints ModuleHints) error {
	if err := validateStringList(section+".include", hints.Include); err != nil {
		return err
	}
	for i, pattern := range hints.ExcludePatterns {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("%s.exclude_patterns[%d] is empty", section, i)
		}
		if _, err := path.Match(pattern, ""); err != nil {
			return fmt.Errorf("%s.exclude_patterns[%d] invalid glob %q: %w", section, i, pattern, err)
		}
	}
	return nil
}

func validateStringList(section string, values []string) error {
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s[%d] is empty", section, i)
		}
	}
	return nil
}

func validateExtras(extras ExplicitExtras) error {
	for i, extra := range extras.Compilers {
		prefix := fmt.Sprintf("extras.compilers[%d]", i)
		if err := requireFields(prefix, map[string]string{
			"module":  extra.Module,
			"name":    extra.Name,
			"version": extra.Version,
			"prefix":  extra.Prefix,
		}); err != nil {
			return err
		}
		if err := requireAbsolutePrefix(prefix+".prefix", extra.Prefix); err != nil {
			return err
		}
		if err := validateLanguages(prefix+".languages", extra.Languages); err != nil {
			return err
		}
	}
	for i, extra := range extras.MPI {
		prefix := fmt.Sprintf("extras.mpi[%d]", i)
		if err := requireFields(prefix, map[string]string{
			"module":     extra.Module,
			"name":       extra.Name,
			"provenance": extra.Provenance,
			"version":    extra.Version,
			"prefix":     extra.Prefix,
		}); err != nil {
			return err
		}
		if err := validateProvenance(prefix+".provenance", extra.Provenance); err != nil {
			return err
		}
		if err := requireAbsolutePrefix(prefix+".prefix", extra.Prefix); err != nil {
			return err
		}
	}
	for i, extra := range extras.GPUToolkits {
		prefix := fmt.Sprintf("extras.gpu_toolkits[%d]", i)
		if err := requireFields(prefix, map[string]string{
			"module":  extra.Module,
			"name":    extra.Name,
			"version": extra.Version,
			"prefix":  extra.Prefix,
		}); err != nil {
			return err
		}
		if err := requireAbsolutePrefix(prefix+".prefix", extra.Prefix); err != nil {
			return err
		}
		for j, component := range extra.SpackComponents {
			componentPrefix := fmt.Sprintf("%s.spack_components[%d]", prefix, j)
			if err := requireFields(componentPrefix, map[string]string{
				"package": component.Package,
				"prefix":  component.Prefix,
			}); err != nil {
				return err
			}
			if err := requireAbsolutePrefix(componentPrefix+".prefix", component.Prefix); err != nil {
				return err
			}
		}
	}
	for i, extra := range extras.FabricUserspace {
		prefix := fmt.Sprintf("extras.fabric_userspace[%d]", i)
		if err := requireFields(prefix, map[string]string{
			"module":  extra.Module,
			"name":    extra.Name,
			"version": extra.Version,
			"prefix":  extra.Prefix,
		}); err != nil {
			return err
		}
		if err := requireAbsolutePrefix(prefix+".prefix", extra.Prefix); err != nil {
			return err
		}
	}
	return nil
}

func requireFields(section string, fields map[string]string) error {
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s.%s is required", section, name)
		}
	}
	return nil
}

func requireAbsolutePrefix(field, value string) error {
	if !filepath.IsAbs(value) {
		return fmt.Errorf("%s must be absolute, got %q", field, value)
	}
	return nil
}

func validateLanguages(field string, languages []string) error {
	if len(languages) == 0 {
		return fmt.Errorf("%s must contain at least one language", field)
	}
	seen := map[string]bool{}
	for i, language := range languages {
		switch language {
		case "c", "c++", "fortran":
		default:
			return fmt.Errorf("%s[%d] invalid language %q", field, i, language)
		}
		if seen[language] {
			return fmt.Errorf("%s[%d] duplicate language %q", field, i, language)
		}
		seen[language] = true
	}
	return nil
}

func validateProvenance(field, provenance string) error {
	switch provenance {
	case "site", "system", "vendor_bundled", "absent":
		return nil
	default:
		return fmt.Errorf("%s invalid provenance %q", field, provenance)
	}
}
