package config

import (
	//"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a YAML configuration file.
//
// Configuration documents must have a mapping as their root node.
func Load(path string) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading YAML file %q: %w", path, err)
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parsing YAML file %q: %w", path, err)
	}

	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil, fmt.Errorf("YAML file %q does not contain a document", path)
	}

	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"YAML file %q must have a mapping as its root node, got %s",
			path,
			nodeKindName(root.Kind),
		)
	}

	return &document, nil
}

// Merge combines overlay into base and returns a new YAML document.
//
// Mapping nodes are merged recursively. Scalar and sequence nodes in the
// overlay completely replace their corresponding value in base.
//
// Neither input document is modified.
func Merge(base, overlay *yaml.Node) (*yaml.Node, error) {
	if err := validateDocument(base, "base"); err != nil {
		return nil, err
	}
	if err := validateDocument(overlay, "overlay"); err != nil {
		return nil, err
	}

	result := cloneNode(base)

	mergeNodes(result.Content[0], overlay.Content[0])

	return result, nil
}

func mergeNodes(base, overlay *yaml.Node) {
	if base.Kind != yaml.MappingNode || overlay.Kind != yaml.MappingNode {
		// This case is only reached recursively for matching map values.
		// The caller replaces the node itself when necessary.
		return
	}

	for i := 0; i < len(overlay.Content); i += 2 {
		overlayKey := overlay.Content[i]
		overlayValue := overlay.Content[i+1]

		baseIndex := findMappingKey(base, overlayKey.Value)

		if baseIndex == -1 {
			base.Content = append(
				base.Content,
				cloneNode(overlayKey),
				cloneNode(overlayValue),
			)
			continue
		}

		baseValue := base.Content[baseIndex+1]

		if baseValue.Kind == yaml.MappingNode &&
			overlayValue.Kind == yaml.MappingNode {
			mergeNodes(baseValue, overlayValue)
			continue
		}

		// Scalars and sequences are replaced wholesale.
		base.Content[baseIndex+1] = cloneNode(overlayValue)
	}
}

func findMappingKey(mapping *yaml.Node, key string) int {
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return i
		}
	}

	return -1
}

func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}

	clone := *node

	clone.Content = make([]*yaml.Node, len(node.Content))
	for i, child := range node.Content {
		clone.Content[i] = cloneNode(child)
	}

	return &clone
}

func validateDocument(document *yaml.Node, name string) error {
	if document == nil {
		return fmt.Errorf("%s YAML document is nil", name)
	}

	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return fmt.Errorf("%s YAML document is invalid", name)
	}

	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf(
			"%s YAML document must have a mapping as its root node, got %s",
			name,
			nodeKindName(root.Kind),
		)
	}

	return nil
}

func nodeKindName(kind yaml.Kind) string {
	switch kind {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}

func EmptyDocument() *yaml.Node {
	return &yaml.Node{
		Kind: yaml.DocumentNode,
		Tag:  "!!doc",
		Content: []*yaml.Node{
			{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
			},
		},
	}
}
