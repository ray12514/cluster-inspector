package model

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ray12514/cluster-inspector/internal/resources"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ValidateProfile validates a Profile against the embedded
// profile-v1.json schema. Returns nil on success or a wrapped error
// describing the first failure.
func ValidateProfile(p *Profile) error {
	if p == nil {
		return fmt.Errorf("validate profile: nil profile")
	}

	doc, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal profile for validation: %w", err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		return fmt.Errorf("decode profile for validation: %w", err)
	}

	schema, err := CompileProfileSchema()
	if err != nil {
		return err
	}
	if err := schema.Validate(instance); err != nil {
		return fmt.Errorf("profile does not match embedded profile schema: %w", err)
	}
	return nil
}

// CompileProfileSchema compiles the embedded canonical profile schema.
func CompileProfileSchema() (*jsonschema.Schema, error) {
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(resources.ProfileSchema))
	if err != nil {
		return nil, fmt.Errorf("parse embedded profile schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("profile-v1.json", schemaDoc); err != nil {
		return nil, fmt.Errorf("load embedded profile schema: %w", err)
	}
	schema, err := compiler.Compile("profile-v1.json")
	if err != nil {
		return nil, fmt.Errorf("compile embedded profile schema: %w", err)
	}
	return schema, nil
}
