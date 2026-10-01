package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func RunnerConfig(
	document *yaml.Node,
) (*yaml.Node, error) {
	root, err := runnerDocumentMapping(document)
	if err != nil {
		return nil, err
	}

	var runner *yaml.Node

	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i]
		value := root.Content[i+1]

		if key.Kind != yaml.ScalarNode ||
			key.Value != "yamr-runner" {
			continue
		}

		if runner != nil {
			return nil, fmt.Errorf(
				"duplicate top-level yamr-runner key",
			)
		}

		runner = value
	}

	if runner == nil ||
		runner.Tag == "!!null" {
		return EmptyDocument(), nil
	}

	if runner.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"top-level yamr-runner must be a mapping, got %s",
			nodeKindName(runner.Kind),
		)
	}

	root = &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: "yamr-runner",
			},
			cloneNode(runner),
		},
	}

	return &yaml.Node{
		Kind: yaml.DocumentNode,
		Tag:  "!!doc",
		Content: []*yaml.Node{
			root,
		},
	}, nil
}

func emptyRunnerDocument() *yaml.Node {
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

func runnerDocumentMapping(
	document *yaml.Node,
) (*yaml.Node, error) {
	if document == nil {
		return nil, fmt.Errorf(
			"YAML document must not be nil",
		)
	}

	if document.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf(
			"expected YAML document node, got %s",
			nodeKindName(document.Kind),
		)
	}

	if len(document.Content) != 1 {
		return nil, fmt.Errorf(
			"expected YAML document to contain exactly one root node",
		)
	}

	root := document.Content[0]

	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"YAML document root must be a mapping, got %s",
			nodeKindName(root.Kind),
		)
	}

	return root, nil
}
