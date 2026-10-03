package runner

import (
	"fmt"
	"path/filepath"
	"strings"

	"jinal--shah/yamr-runner/internal/action"
	"jinal--shah/yamr-runner/internal/docker"

	"gopkg.in/yaml.v3"
)

type OutputDestination struct {
	Path    string
	Inherit bool
}

type PlannedOnFail struct {
	Docker docker.Command
	Stdout OutputDestination
	Stderr OutputDestination
}

// CompileOnFail compiles the optional yamr-runner.action.on_fail
// configuration.
//
// A nil result means that the action has no on_fail configuration.
//
// on_fail deliberately does not support pre_run. The runner itself
// creates the .yamr-debug directory before executing on_fail.
func CompileOnFail(
	a *action.Action,
) (*PlannedOnFail, error) {
	if a == nil {
		return nil, fmt.Errorf(
			"action must not be nil",
		)
	}

	if a.Config == nil {
		return nil, fmt.Errorf(
			"action %q has nil config",
			a.ActionFile,
		)
	}

	actionNode, err := actionConfigNode(a.Config)
	if err != nil {
		return nil, onFailActionError(a, err)
	}

	onFail := mappingValue(
		actionNode,
		"on_fail",
	)

	if onFail == nil || onFail.Tag == "!!null" {
		return nil, nil
	}

	if onFail.Kind != yaml.MappingNode {
		return nil, onFailActionError(
			a,
			fmt.Errorf(
				"yamr-runner.action.on_fail must be a mapping",
			),
		)
	}

	if preRun := mappingValue(
		onFail,
		"pre_run",
	); preRun != nil {
		return nil, onFailActionError(
			a,
			fmt.Errorf(
				"yamr-runner.action.on_fail.pre_run "+
					"is not supported",
			),
		)
	}

	if err := validateMappingKeys(
		onFail,
		"run",
	); err != nil {
		return nil, onFailActionError(
			a,
			fmt.Errorf(
				"yamr-runner.action.on_fail: %w",
				err,
			),
		)
	}

	run := mappingValue(
		onFail,
		"run",
	)
	if run == nil {
		return nil, onFailActionError(
			a,
			fmt.Errorf(
				"yamr-runner.action.on_fail.run is required",
			),
		)
	}

	if run.Kind != yaml.MappingNode {
		return nil, onFailActionError(
			a,
			fmt.Errorf(
				"yamr-runner.action.on_fail.run "+
					"must be a mapping",
			),
		)
	}

	if err := validateMappingKeys(
		run,
		"docker",
		"stdout",
		"stderr",
	); err != nil {
		return nil, onFailActionError(
			a,
			fmt.Errorf(
				"yamr-runner.action.on_fail.run: %w",
				err,
			),
		)
	}

	if mappingValue(run, "docker") == nil {
		return nil, onFailActionError(
			a,
			fmt.Errorf(
				"yamr-runner.action.on_fail.run.docker "+
					"is required",
			),
		)
	}

	dockerCommand, err := compileOnFailDocker(
		a,
		run,
	)
	if err != nil {
		return nil, onFailActionError(
			a,
			err,
		)
	}

	debugDir := filepath.Join(
		a.ActionDir,
		".yamr-debug",
	)

	stdout, err := compileOnFailOutput(
		mappingValue(run, "stdout"),
		filepath.Join(
			debugDir,
			"stdout.log",
		),
	)
	if err != nil {
		return nil, onFailActionError(
			a,
			fmt.Errorf(
				"yamr-runner.action.on_fail.run.stdout: %w",
				err,
			),
		)
	}

	stderr, err := compileOnFailOutput(
		mappingValue(run, "stderr"),
		filepath.Join(
			debugDir,
			"stderr.log",
		),
	)
	if err != nil {
		return nil, onFailActionError(
			a,
			fmt.Errorf(
				"yamr-runner.action.on_fail.run.stderr: %w",
				err,
			),
		)
	}

	return &PlannedOnFail{
		Docker: dockerCommand,
		Stdout: stdout,
		Stderr: stderr,
	}, nil
}

// compileOnFailDocker reuses docker.Compile rather than duplicating
// Docker configuration parsing.
//
// docker.Compile currently reads:
//
//	yamr-runner.action.run.docker
//
// Construct a small synthetic action document in which the on_fail
// run node becomes the normal run node.
func compileOnFailDocker(
	a *action.Action,
	run *yaml.Node,
) (docker.Command, error) {
	document := &yaml.Node{
		Kind: yaml.DocumentNode,
		Tag:  "!!doc",
	}

	root := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}

	runner := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}

	actionMapping := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}

	root.Content = append(
		root.Content,
		stringNode("yamr-runner"),
		runner,
	)

	runner.Content = append(
		runner.Content,
		stringNode("action"),
		actionMapping,
	)

	actionMapping.Content = append(
		actionMapping.Content,
		stringNode("run"),
		cloneYAMLNode(run),
	)

	document.Content = append(
		document.Content,
		root,
	)

	synthetic := &action.Action{
		ActionFile: a.ActionFile,
		ActionDir:  a.ActionDir,
		Config:     document,
		Env:        cloneStringMap(a.Env),
		Sources:    append([]string(nil), a.Sources...),
	}

	command, err := docker.Compile(synthetic)
	if err != nil {
		return docker.Command{}, fmt.Errorf(
			"compile yamr-runner.action.on_fail.run.docker: %w",
			err,
		)
	}

	return command, nil
}

// compileOnFailOutput applies the on_fail output rules:
//
//   - absent: use the supplied default .yamr-debug log path
//   - null:   inherit the runner's stdout/stderr
//   - string: redirect to that path
//
// Tokens have already been resolved before action compilation, so an
// explicit path is execution-ready here.
func compileOnFailOutput(
	node *yaml.Node,
	defaultPath string,
) (OutputDestination, error) {
	if node == nil {
		return OutputDestination{
			Path: defaultPath,
		}, nil
	}

	if node.Tag == "!!null" {
		return OutputDestination{
			Inherit: true,
		}, nil
	}

	if node.Kind != yaml.ScalarNode ||
		node.Tag != "!!str" {
		return OutputDestination{}, fmt.Errorf(
			"must be a string or null",
		)
	}

	path := strings.TrimSpace(node.Value)
	if path == "" {
		return OutputDestination{}, fmt.Errorf(
			"must not be empty",
		)
	}

	if !filepath.IsAbs(path) {
		return OutputDestination{}, fmt.Errorf(
			"path %q must be absolute",
			path,
		)
	}

	return OutputDestination{
		Path: filepath.Clean(path),
	}, nil
}

func actionConfigNode(
	document *yaml.Node,
) (*yaml.Node, error) {
	root, err := documentMapping(document)
	if err != nil {
		return nil, err
	}

	runner := mappingValue(
		root,
		"yamr-runner",
	)
	if runner == nil ||
		runner.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"yamr-runner must be a mapping",
		)
	}

	actionNode := mappingValue(
		runner,
		"action",
	)
	if actionNode == nil ||
		actionNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"yamr-runner.action must be a mapping",
		)
	}

	return actionNode, nil
}

func stringNode(
	value string,
) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}
}

func cloneYAMLNode(
	node *yaml.Node,
) *yaml.Node {
	if node == nil {
		return nil
	}

	cloned := *node

	if node.Content != nil {
		cloned.Content = make(
			[]*yaml.Node,
			len(node.Content),
		)

		for i, child := range node.Content {
			cloned.Content[i] = cloneYAMLNode(
				child,
			)
		}
	}

	return &cloned
}

func cloneStringMap(
	values map[string]string,
) map[string]string {
	if values == nil {
		return nil
	}

	cloned := make(
		map[string]string,
		len(values),
	)

	for key, value := range values {
		cloned[key] = value
	}

	return cloned
}

func onFailActionError(
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
