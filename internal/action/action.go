package action

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"jinal--shah/yamr-runner/internal/discover"
	"jinal--shah/yamr-runner/internal/sources"
	"jinal--shah/yamr-runner/internal/tokens"
)

type Action struct {
	ActionFile string
	ActionDir  string

	Config *yaml.Node

	// Env is only the environment explicitly configured through YAML.
	// These are the variables that will later be passed to Docker.
	Env map[string]string

	// Sources is the final resolved source list.
	Sources []string
}

type Options struct {
	Candidate discover.Candidate
	Labels    *sources.Labels

	// HostEnv should normally be os.Environ().
	HostEnv []string
}

func Compile(options Options) (*Action, error) {
	candidate := options.Candidate

	if candidate.Config == nil {
		return nil, fmt.Errorf(
			"candidate %q has nil configuration",
			candidate.ActionFile,
		)
	}

	runner, err := runnerConfig(candidate.Config)
	if err != nil {
		return nil, candidateError(candidate, err)
	}

	tokenEnv, err := tokens.BuildEnvironment(
		candidate.Config,
		options.HostEnv,
	)
	if err != nil {
		return nil, candidateError(candidate, err)
	}

	inlineSources, hasInlineSources, err :=
		optionalStringSequence(runner, "sources")
	if err != nil {
		return nil, candidateError(candidate, err)
	}

	sourceLabel, hasSourceLabel, err :=
		optionalString(runner, "sources-label")
	if err != nil {
		return nil, candidateError(candidate, err)
	}

	var (
		selectedSources []string
		sourceToken     string
	)

	switch {
	case hasInlineSources:
		usesInlineToken, err := dockerCmdSourcesIsToken(
			runner,
			"$yamr_sources$",
		)
		if err != nil {
			return nil, candidateError(candidate, err)
		}

		if !usesInlineToken {
			return nil, candidateError(
				candidate,
				fmt.Errorf(
					"yamr-runner.sources is configured, so "+
						"yamr-runner.action.run.docker.cmd_sources "+
						"must be $yamr_sources$",
				),
			)
		}

		selectedSources = inlineSources
		sourceToken = "yamr_sources"

	case hasSourceLabel:
		usesLabelToken, err := dockerCmdSourcesIsToken(
			runner,
			"$yamr_sources_from_label$",
		)
		if err != nil {
			return nil, candidateError(candidate, err)
		}

		if !usesLabelToken {
			return nil, candidateError(
				candidate,
				fmt.Errorf(
					"yamr-runner.sources-label is configured, so "+
						"yamr-runner.action.run.docker.cmd_sources "+
						"must be $yamr_sources_from_label$",
				),
			)
		}

		if options.Labels == nil {
			return nil, candidateError(
				candidate,
				fmt.Errorf(
					"sources-label %q requires loaded source labels",
					sourceLabel,
				),
			)
		}

		selectedSources, err = options.Labels.Lookup(sourceLabel)
		if err != nil {
			return nil, candidateError(candidate, err)
		}

		sourceToken = "yamr_sources_from_label"

	default:
		return nil, candidateError(
			candidate,
			fmt.Errorf(
				"action must define yamr-runner.sources "+
					"or yamr-runner.sources-label",
			),
		)
	}

	if len(selectedSources) == 0 {
		return nil, candidateError(
			candidate,
			fmt.Errorf(
				"selected yamr source list must not be empty",
			),
		)
	}

	selectedSources, err = resolveSources(
		selectedSources,
		tokenEnv,
		candidate.ActionDir,
	)
	if err != nil {
		return nil, candidateError(candidate, err)
	}

	finalConfig := cloneNode(candidate.Config)

	tokenContext := tokens.Context{
		ActionDir: candidate.ActionDir,
		Env:       tokenEnv,
	}

	switch sourceToken {
	case "yamr_sources":
		tokenContext.YamrSources = selectedSources

	case "yamr_sources_from_label":
		tokenContext.YamrSourcesFromLabel = selectedSources
	}

	err = tokens.ResolveDocument(
		finalConfig,
		tokenContext,
		tokens.Final,
	)
	if err != nil {
		return nil, candidateError(candidate, err)
	}

	finalRunner, err := runnerConfig(finalConfig)
	if err != nil {
		return nil, candidateError(candidate, err)
	}

	configuredEnv, err := configuredEnvironment(finalRunner)
	if err != nil {
		return nil, candidateError(candidate, err)
	}

	return &Action{
		ActionFile: candidate.ActionFile,
		ActionDir:  candidate.ActionDir,
		Config:     finalConfig,
		Env:        configuredEnv,
		Sources:    selectedSources,
	}, nil
}

func runnerConfig(document *yaml.Node) (*yaml.Node, error) {
	if document.Kind != yaml.DocumentNode ||
		len(document.Content) != 1 {
		return nil, fmt.Errorf("invalid YAML document")
	}

	root := document.Content[0]

	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("configuration root must be a mapping")
	}

	runner := mappingValue(root, "yamr-runner")
	if runner == nil {
		return nil, fmt.Errorf("yamr-runner configuration is missing")
	}

	if runner.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("yamr-runner must be a mapping")
	}

	return runner, nil
}

func optionalBool(
	mapping *yaml.Node,
	key string,
) (bool, error) {
	node := mappingValue(mapping, key)
	if node == nil {
		return false, nil
	}

	if node.Kind != yaml.ScalarNode ||
		node.Tag != "!!bool" ||
		(node.Value != "true" && node.Value != "false") {
		return false, fmt.Errorf(
			"yamr-runner.%s must be true or false",
			key,
		)
	}

	return node.Value == "true", nil
}

func optionalString(
	mapping *yaml.Node,
	key string,
) (string, bool, error) {
	node := mappingValue(mapping, key)
	if node == nil {
		return "", false, nil
	}

	if node.Kind != yaml.ScalarNode ||
		node.Tag != "!!str" {
		return "", false, fmt.Errorf(
			"yamr-runner.%s must be a string",
			key,
		)
	}

	if node.Value == "" {
		return "", false, fmt.Errorf(
			"yamr-runner.%s must not be empty",
			key,
		)
	}

	return node.Value, true, nil
}

func optionalStringSequence(
	mapping *yaml.Node,
	key string,
) ([]string, bool, error) {
	node := mappingValue(mapping, key)
	if node == nil {
		return nil, false, nil
	}

	if node.Kind != yaml.SequenceNode {
		return nil, false, fmt.Errorf(
			"yamr-runner.%s must be a sequence",
			key,
		)
	}

	result := make([]string, 0, len(node.Content))

	for i, child := range node.Content {
		if child.Kind != yaml.ScalarNode ||
			child.Tag != "!!str" {
			return nil, false, fmt.Errorf(
				"yamr-runner.%s[%d] must be a string",
				key,
				i,
			)
		}

		result = append(result, child.Value)
	}

	return result, true, nil
}

func mappingValue(
	mapping *yaml.Node,
	key string,
) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}

	return nil
}

func dockerCmdSourcesIsToken(
	runner *yaml.Node,
	token string,
) (bool, error) {
	action := mappingValue(runner, "action")
	if action == nil {
		return false, nil
	}

	if action.Kind != yaml.MappingNode {
		return false, fmt.Errorf(
			"yamr-runner.action must be a mapping",
		)
	}

	run := mappingValue(action, "run")
	if run == nil {
		return false, nil
	}

	if run.Kind != yaml.MappingNode {
		return false, fmt.Errorf(
			"yamr-runner.action.run must be a mapping",
		)
	}

	docker := mappingValue(run, "docker")
	if docker == nil {
		return false, nil
	}

	if docker.Kind != yaml.MappingNode {
		return false, fmt.Errorf(
			"yamr-runner.action.run.docker must be a mapping",
		)
	}

	cmdSources := mappingValue(docker, "cmd_sources")
	if cmdSources == nil {
		return false, nil
	}

	if cmdSources.Kind != yaml.ScalarNode {
		return false, nil
	}

	return cmdSources.Value == token, nil
}

func configuredEnvironment(
	runner *yaml.Node,
) (map[string]string, error) {
	result := make(map[string]string)

	env := mappingValue(runner, "env")
	if env == nil {
		return result, nil
	}

	if env.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"yamr-runner.env must be a mapping",
		)
	}

	for i := 0; i < len(env.Content); i += 2 {
		key := env.Content[i].Value
		value := env.Content[i+1]

		if value.Tag == "!!null" {
			continue
		}

		if value.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf(
				"yamr-runner.env.%s must be a scalar or null",
				key,
			)
		}

		result[key] = value.Value
	}

	return result, nil
}

func resolveSources(
	values []string,
	env map[string]string,
	actionDir string,
) ([]string, error) {
	result := make([]string, len(values))

	for i, value := range values {
		resolved, err := tokens.ResolveString(
			value,
			tokens.Context{
				ActionDir: actionDir,
				Env:       env,
			},
			tokens.Final,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"resolving source %q: %w",
				value,
				err,
			)
		}

		result[i] = resolved
	}

	return result, nil
}

func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}

	result := *node

	result.Content = make(
		[]*yaml.Node,
		len(node.Content),
	)

	for i, child := range node.Content {
		result.Content[i] = cloneNode(child)
	}

	return &result
}

func candidateError(
	candidate discover.Candidate,
	err error,
) error {
	return fmt.Errorf(
		"action %q: %w",
		candidate.ActionFile,
		err,
	)
}
