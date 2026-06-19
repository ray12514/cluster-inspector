package output

import (
	"io"

	"github.com/ray12514/cluster-inspector/internal/model"
	"gopkg.in/yaml.v3"
)

// WriteSystemFragment emits a system fragment in reviewable YAML form.
func WriteSystemFragment(w io.Writer, fragment *model.SystemFragment) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(fragment); err != nil {
		_ = enc.Close()
		return err
	}
	return enc.Close()
}
