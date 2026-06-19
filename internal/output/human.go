package output

import (
	"fmt"
	"io"
	"sort"

	"github.com/ray12514/cluster-inspector/internal/model"
)

// WriteSummary emits a short human-readable profile summary for diagnostics.
func WriteSummary(w io.Writer, p *model.Profile) error {
	if p == nil {
		return fmt.Errorf("write summary: nil profile")
	}

	nodeNames := make([]string, 0, len(p.NodeTypes))
	for name := range p.NodeTypes {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	if _, err := fmt.Fprintf(w, "system: %s\n", p.System.Name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "family: %s\n", p.System.Family); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "os: %s %d\n", p.OS.Name, p.OS.Major); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "fabric: %s\n", p.Fabric.Type); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "node_types: %v\n", nodeNames)
	return err
}
