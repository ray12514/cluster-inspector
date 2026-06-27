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

// DiscoveryPolicy contains generic Linux discovery policy plus platform
// adapter hints. Probes execute this policy; they should not own provider
// vocabulary or standard filesystem roots directly.
//
//go:embed discovery_policy.yaml
var DiscoveryPolicy []byte
