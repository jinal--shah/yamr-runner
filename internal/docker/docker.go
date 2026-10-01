package docker

import (
	"fmt"

	"jinal--shah/yamr-run/internal/action"

	"gopkg.in/yaml.v3"
)

// Compile extracts and validates the Docker configuration from a fully
// compiled action.
func Compile(a *action.Action) (Command, error) {
	if a == nil {
		return Command{}, fmt.Errorf("action must not be nil")
	}

	if a.Config == nil {
		return Command{}, fmt.Errorf(
			"action %q has nil config",
			a.ActionFile,
		)
	}

	docker, err := dockerConfig(a.Config)
	if err != nil {
		return Command{}, actionError(a, err)
	}

	image, err := requiredString(
		docker,
		"image",
		"yamr-runner.action.run.docker.image",
	)
	if err != nil {
		return Command{}, actionError(a, err)
	}

	entrypointValues, err := requiredStringSequence(
		docker,
		"entrypoint",
		"yamr-runner.action.run.docker.entrypoint",
	)
	if err != nil {
		return Command{}, actionError(a, err)
	}

	if len(entrypointValues) != 1 {
		return Command{}, actionError(
			a,
			fmt.Errorf(
				"yamr-runner.action.run.docker.entrypoint "+
					"must contain exactly one string",
			),
		)
	}

	cmdOpts, err := optionalStringSequence(
		docker,
		"cmd_opts",
		"yamr-runner.action.run.docker.cmd_opts",
	)
	if err != nil {
		return Command{}, actionError(a, err)
	}

	cmdSources, err := requiredStringSequence(
		docker,
		"cmd_sources",
		"yamr-runner.action.run.docker.cmd_sources",
	)
	if err != nil {
		return Command{}, actionError(a, err)
	}

	if len(cmdSources) == 0 {
		return Command{}, actionError(
			a,
			fmt.Errorf(
				"yamr-runner.action.run.docker.cmd_sources "+
					"must contain at least one source",
			),
		)
	}

	userGroup, err := optionalString(
		docker,
		"user_group",
		"yamr-runner.action.run.docker.user_group",
	)
	if err != nil {
		return Command{}, actionError(a, err)
	}

	workDir, err := optionalString(
		docker,
		"work_dir",
		"yamr-runner.action.run.docker.work_dir",
	)
	if err != nil {
		return Command{}, actionError(a, err)
	}

	mounts, err := optionalStringSequence(
		docker,
		"mounts",
		"yamr-runner.action.run.docker.mounts",
	)
	if err != nil {
		return Command{}, actionError(a, err)
	}

	return Command{
		Image:      image,
		Entrypoint: entrypointValues[0],
		CmdOpts:    cmdOpts,
		CmdSources: cmdSources,
		UserGroup:  userGroup,
		WorkDir:    workDir,
		Mounts:     mounts,
		Env:        cloneStringMap(a.Env),
	}, nil
}

func dockerConfig(
	document *yaml.Node,
) (*yaml.Node, error) {
	root, err := documentMapping(document)
	if err != nil {
		return nil, err
	}

	runner := mappingValue(root, "yamr-runner")
	if runner == nil {
		return nil, fmt.Errorf(
			"config has no yamr-runner mapping",
		)
	}

	if runner.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"yamr-runner must be a mapping",
		)
	}

	actionNode := mappingValue(runner, "action")
	if actionNode == nil {
		return nil, fmt.Errorf(
			"yamr-runner.action is required",
		)
	}

	if actionNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"yamr-runner.action must be a mapping",
		)
	}

	run := mappingValue(actionNode, "run")
	if run == nil {
		return nil, fmt.Errorf(
			"yamr-runner.action.run is required",
		)
	}

	if run.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"yamr-runner.action.run must be a mapping",
		)
	}

	docker := mappingValue(run, "docker")
	if docker == nil {
		return nil, fmt.Errorf(
			"yamr-runner.action.run.docker is required",
		)
	}

	if docker.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"yamr-runner.action.run.docker must be a mapping",
		)
	}

	return docker, nil
}

func requiredString(
	mapping *yaml.Node,
	key string,
	path string,
) (string, error) {
	node := mappingValue(mapping, key)
	if node == nil {
		return "", fmt.Errorf("%s is required", path)
	}

	if node.Kind != yaml.ScalarNode ||
		node.Tag == "!!null" {
		return "", fmt.Errorf(
			"%s must be a string",
			path,
		)
	}

	if node.Value == "" {
		return "", fmt.Errorf(
			"%s must not be empty",
			path,
		)
	}

	return node.Value, nil
}

func optionalString(
	mapping *yaml.Node,
	key string,
	path string,
) (string, error) {
	node := mappingValue(mapping, key)
	if node == nil || node.Tag == "!!null" {
		return "", nil
	}

	if node.Kind != yaml.ScalarNode {
		return "", fmt.Errorf(
			"%s must be a string",
			path,
		)
	}

	return node.Value, nil
}

func requiredStringSequence(
	mapping *yaml.Node,
	key string,
	path string,
) ([]string, error) {
	node := mappingValue(mapping, key)
	if node == nil {
		return nil, fmt.Errorf("%s is required", path)
	}

	return stringSequence(node, path)
}

func optionalStringSequence(
	mapping *yaml.Node,
	key string,
	path string,
) ([]string, error) {
	node := mappingValue(mapping, key)
	if node == nil || node.Tag == "!!null" {
		return nil, nil
	}

	return stringSequence(node, path)
}

func stringSequence(
	node *yaml.Node,
	path string,
) ([]string, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf(
			"%s must be a sequence",
			path,
		)
	}

	values := make([]string, 0, len(node.Content))

	for i, item := range node.Content {
		if item.Kind != yaml.ScalarNode ||
			item.Tag == "!!null" {
			return nil, fmt.Errorf(
				"%s[%d] must be a string",
				path,
				i,
			)
		}

		values = append(values, item.Value)
	}

	return values, nil
}

func documentMapping(
	document *yaml.Node,
) (*yaml.Node, error) {
	if document == nil {
		return nil, fmt.Errorf("config is nil")
	}

	if document.Kind != yaml.DocumentNode ||
		len(document.Content) != 1 {
		return nil, fmt.Errorf(
			"config must be a YAML document",
		)
	}

	root := document.Content[0]

	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"config root must be a mapping",
		)
	}

	return root, nil
}

func mappingValue(
	mapping *yaml.Node,
	key string,
) *yaml.Node {
	if mapping == nil ||
		mapping.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}

	return nil
}

func cloneStringMap(
	value map[string]string,
) map[string]string {
	if value == nil {
		return nil
	}

	result := make(
		map[string]string,
		len(value),
	)

	for key, value := range value {
		result[key] = value
	}

	return result
}

func actionError(
	a *action.Action,
	err error,
) error {
	if a.ActionFile == "" {
		return err
	}

	return fmt.Errorf(
		"action %q: %w",
		a.ActionFile,
		err,
	)
}
