// Package output handles emitting profiles and fragments in stable,
// reviewable form. The YAML emitter is the durable output; JSON and
// human emitters are for diagnostics.
package output

import (
	"io"
	"sort"

	"github.com/ray12514/cluster-inspector/internal/model"
	"gopkg.in/yaml.v3"
)

// WriteProfile emits a profile.yaml with a stable key order optimised
// for human review (matching the field order in profile-v1.json), not
// insertion order. Two calls with the same input must produce
// byte-identical output.
func WriteProfile(w io.Writer, p *model.Profile) error {
	node, err := profileNode(p)
	if err != nil {
		return err
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		_ = enc.Close()
		return err
	}
	return enc.Close()
}

func profileNode(p *model.Profile) (*yaml.Node, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	if err := appendValue(root, "schema_version", p.SchemaVersion); err != nil {
		return nil, err
	}
	if err := appendValue(root, "system", p.System); err != nil {
		return nil, err
	}
	if err := appendValue(root, "os", p.OS); err != nil {
		return nil, err
	}
	if err := appendValue(root, "fabric", p.Fabric); err != nil {
		return nil, err
	}
	if err := appendValue(root, "modules_system", p.ModulesSystem); err != nil {
		return nil, err
	}
	if len(p.CompilerProviders) > 0 {
		if err := appendValue(root, "compiler_providers", p.CompilerProviders); err != nil {
			return nil, err
		}
	}
	if len(p.MPIProviders) > 0 {
		if err := appendValue(root, "mpi_providers", p.MPIProviders); err != nil {
			return nil, err
		}
	}
	if p.GPUToolkitModules != nil {
		if err := appendValue(root, "gpu_toolkit_modules", p.GPUToolkitModules); err != nil {
			return nil, err
		}
	}
	if err := appendValue(root, "filesystem", p.Filesystem); err != nil {
		return nil, err
	}
	if err := appendNode(root, "node_types", nodeTypesNode(p.NodeTypes)); err != nil {
		return nil, err
	}
	return root, nil
}

func nodeTypesNode(nodeTypes map[string]model.NodeType) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}
	names := make([]string, 0, len(nodeTypes))
	for name := range nodeTypes {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		child := &yaml.Node{}
		if err := child.Encode(nodeTypes[name]); err != nil {
			continue
		}
		_ = appendNode(node, name, child)
	}
	return node
}

func appendValue(mapping *yaml.Node, key string, value any) error {
	node := &yaml.Node{}
	if err := node.Encode(value); err != nil {
		return err
	}
	return appendNode(mapping, key, node)
}

func appendNode(mapping *yaml.Node, key string, value *yaml.Node) error {
	mapping.Content = append(mapping.Content, &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: key,
	}, value)
	return nil
}
