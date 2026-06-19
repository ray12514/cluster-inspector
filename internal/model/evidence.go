package model

// Confidence captures the inspector's confidence in a probed fact.
// Values mirror the design doc's confidence vocabulary.
type Confidence string

const (
	ConfidenceProbed   Confidence = "probed"
	ConfidenceInferred Confidence = "inferred"
	ConfidenceUnknown  Confidence = "unknown"
)

// Evidence records why the inspector believes a fact. Inline compact
// evidence may surface in the durable profile (confidence + short source
// string); full command output belongs in an optional diagnostic
// artifact, not in profile.yaml.
type Evidence struct {
	Confidence Confidence `json:"confidence,omitempty" yaml:"confidence,omitempty"`
	Source     string     `json:"source,omitempty" yaml:"source,omitempty"`
}
