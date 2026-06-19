package output

import (
	"encoding/json"
	"io"
)

// WriteJSON emits diagnostic data as stable, indented JSON.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
