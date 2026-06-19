package resources

import _ "embed"

// ProfileSchema is the canonical profile-v1 JSON Schema.
//
//go:embed profile_schema.json
var ProfileSchema []byte

// ModulePatterns contains module-name classification patterns used during
// candidate discovery.
//
//go:embed module_patterns.yaml
var ModulePatterns []byte
