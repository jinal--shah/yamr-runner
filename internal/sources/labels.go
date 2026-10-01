package sources

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Labels struct {
	values map[string][]string
}

func LoadLabels(path string) (*Labels, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"reading yamr source labels file %q: %w",
			path,
			err,
		)
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf(
			"parsing yamr source labels file %q: %w",
			path,
			err,
		)
	}

	if document.Kind != yaml.DocumentNode ||
		len(document.Content) != 1 {
		return nil, fmt.Errorf(
			"yamr source labels file %q does not contain a YAML document",
			path,
		)
	}

	root := document.Content[0]

	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"yamr source labels file %q must have a mapping as its root",
			path,
		)
	}

	labels := &Labels{
		values: make(map[string][]string),
	}

	for i := 0; i < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valueNode := root.Content[i+1]

		if keyNode.Kind != yaml.ScalarNode ||
			keyNode.Tag != "!!str" {
			return nil, fmt.Errorf(
				"yamr source labels file %q contains a non-string label",
				path,
			)
		}

		label := keyNode.Value

		if label == "" {
			return nil, fmt.Errorf(
				"yamr source labels file %q contains an empty label",
				path,
			)
		}

		if _, exists := labels.values[label]; exists {
			return nil, fmt.Errorf(
				"yamr source labels file %q contains duplicate label %q",
				path,
				label,
			)
		}

		if valueNode.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf(
				"yamr source label %q in %q must contain a sequence",
				label,
				path,
			)
		}

		values := make([]string, 0, len(valueNode.Content))

		for index, sourceNode := range valueNode.Content {
			if sourceNode.Kind != yaml.ScalarNode ||
				sourceNode.Tag != "!!str" {
				return nil, fmt.Errorf(
					"yamr source label %q in %q contains "+
						"a non-string value at index %d",
					label,
					path,
					index,
				)
			}

			values = append(values, sourceNode.Value)
		}

		labels.values[label] = values
	}

	return labels, nil
}

func (l *Labels) Lookup(label string) ([]string, error) {
	if l == nil {
		return nil, fmt.Errorf("yamr source labels are not loaded")
	}

	values, exists := l.values[label]
	if !exists {
		return nil, fmt.Errorf(
			"yamr source label %q does not exist",
			label,
		)
	}

	// Don't expose our internal slice to callers.
	result := make([]string, len(values))
	copy(result, values)

	return result, nil
}
