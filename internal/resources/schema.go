package resources

import _ "embed"

// ProfileSchema is the canonical profile-v1 JSON Schema.
//
//go:embed profile_schema.json
var ProfileSchema []byte
