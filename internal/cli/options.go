package cli

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"jinal--shah/yamr-runner/internal/config"
)

const DefaultMaxWorkers = 4

type Options struct {
	ConfigFile       string
	YamrSourcesDir   string
	YamrSourceLabels string
	NoPrompts        bool
	MaxWorkers       int
}

// stringOption
type stringOption struct {
	value string
	set   bool
}

func (o *stringOption) String() string {
	return o.value
}

func (o *stringOption) Set(value string) error {
	o.value = value
	o.set = true
	return nil
}

// intOption
type intOption struct {
	value int
	set   bool
}

func (o *intOption) Set(
	value string,
) error {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf(
			"invalid integer %q",
			value,
		)
	}

	o.value = parsed
	o.set = true

	return nil
}

func (o *intOption) String() string {
	return strconv.Itoa(o.value)
}

// abstract the actual loading of the config, because I want
// to test the cli bit without opening a file or creating yaml to support that.
type configLoaderFunc func(
	string,
) (*yaml.Node, error)

func Parse(
	args []string,
	environ []string,
) (Options, error) {
	return parse(
		args,
		environ,
		config.Load,
	)
}

func parse(
	args []string,
	environ []string,
	loadConfig configLoaderFunc,
) (Options, error) {

	env := parseEnvironment(environ)

	flags := flag.NewFlagSet(
		"yamr-runner",
		flag.ContinueOnError,
	)

	flags.SetOutput(io.Discard)

	var configFile stringOption
	var noPrompts bool
	var maxWorkers intOption
	var yamrSourcesDir stringOption
	var yamrSourceLabels stringOption

	flags.Var(
		&configFile,
		"config",
		"runner configuration file",
	)

	flags.Var(
		&configFile,
		"c",
		"runner configuration file",
	)

	flags.BoolVar(
		&noPrompts,
		"no-prompts",
		false,
		"run without interactive prompts",
	)

	flags.Var(
		&yamrSourcesDir,
		"yamr-sources-dir",
		"yamr sources directory",
	)

	flags.Var(
		&yamrSourceLabels,
		"yamr-source-labels",
		"yamr source labels file",
	)

	flags.Var(
		&maxWorkers,
		"max-workers",
		"maximum number of actions to run concurrently",
	)

	if err := flags.Parse(args); err != nil {
		return Options{}, err
	}

	if flags.NArg() != 0 {
		return Options{}, fmt.Errorf(
			"unexpected arguments: %s",
			strings.Join(flags.Args(), " "),
		)
	}

	configPath, err := requiredCLIOrEnv(
		"config",
		configFile,
		env,
		"YAMR_RUNNER_CONFIG",
	)
	if err != nil {
		return Options{}, err
	}

	document, err := loadConfig(configPath)
	if err != nil {
		return Options{}, fmt.Errorf(
			"load runner config %q: %w",
			configPath,
			err,
		)
	}

	sourcesDir, err := resolveString(
		"yamr-sources-dir",
		yamrSourcesDir,
		env,
		"YAMR_SOURCES_DIR",
		document,
		"yamr-sources-dir",
	)
	if err != nil {
		return Options{}, err
	}

	sourceLabels, err := resolveString(
		"yamr-source-labels",
		yamrSourceLabels,
		env,
		"YAMR_SOURCE_LABELS",
		document,
		"yamr-source-labels",
	)
	if err != nil {
		return Options{}, err
	}

	resolvedMaxWorkers, err := resolveMaxWorkers(
		maxWorkers,
		"max-workers",
		env,
		"YAMR_RUNNER_MAX_WORKERS",
	)
	if err != nil {
		return Options{}, err
	}

	return Options{
		ConfigFile:       configPath,
		NoPrompts:        noPrompts,
		YamrSourcesDir:   sourcesDir,
		YamrSourceLabels: sourceLabels,
		MaxWorkers:       resolvedMaxWorkers,
	}, nil
}

func requiredCLIOrEnv(
	name string,
	option stringOption,
	env map[string]string,
	envName string,
) (string, error) {
	if option.set {
		if option.value == "" {
			return "", fmt.Errorf(
				"--%s must not be empty",
				name,
			)
		}

		return option.value, nil
	}

	if value, ok := env[envName]; ok {
		if value == "" {
			return "", fmt.Errorf(
				"%s must not be empty",
				envName,
			)
		}

		return value, nil
	}

	return "", fmt.Errorf(
		"--%s or %s is required",
		name,
		envName,
	)
}

func resolveMaxWorkers(
	option intOption,
	cmdLineOpt string,
	env map[string]string,
	envName string,
) (int, error) {
	value := DefaultMaxWorkers
	from := "default"

	if option.set {
		value = option.value
		from = fmt.Sprintf(
			"--%s",
			cmdLineOpt,
		)
	} else if envValue, ok := env[envName]; ok {
		if envValue == "" {
			return value, fmt.Errorf(
				"%s must not be empty",
				envName,
			)
		}

		intValue, err := strconv.Atoi(envValue)
		if err != nil {
			return value, fmt.Errorf(
				"%s must be an integer: %q",
				envName,
				envValue,
			)
		}

		value = intValue
		from = fmt.Sprintf(
			"envvar %s",
			envName,
		)
	}

	if value < 1 || value > 8 {
		return value, fmt.Errorf(
			"%s must be between 1 and 8",
			from,
		)
	}

	return value, nil
}

func resolveString(
	name string,
	option stringOption,
	env map[string]string,
	envName string,
	document *yaml.Node,
	configKey string,
) (string, error) {
	if option.set {
		if option.value == "" {
			return "", fmt.Errorf(
				"--%s must not be empty",
				name,
			)
		}

		return option.value, nil
	}

	if value, ok := env[envName]; ok {
		if value == "" {
			return "", fmt.Errorf(
				"%s must not be empty",
				envName,
			)
		}

		return value, nil
	}

	value, found, err := topLevelString(
		document,
		configKey,
	)
	if err != nil {
		return "", err
	}

	if found {
		if value == "" {
			return "", fmt.Errorf(
				"top-level %s must not be empty",
				configKey,
			)
		}

		return value, nil
	}

	return "", fmt.Errorf(
		"%s is required via --%s, %s, or top-level %s",
		name,
		name,
		envName,
		configKey,
	)
}

func parseEnvironment(
	environ []string,
) map[string]string {
	result := make(map[string]string)

	for _, entry := range environ {
		key, value, found := strings.Cut(
			entry,
			"=",
		)
		if !found {
			continue
		}

		result[key] = value
	}

	return result
}

func topLevelString(
	document *yaml.Node,
	key string,
) (string, bool, error) {
	if document == nil ||
		document.Kind != yaml.DocumentNode ||
		len(document.Content) != 1 {
		return "", false, fmt.Errorf(
			"invalid runner configuration document",
		)
	}

	root := document.Content[0]

	if root.Kind != yaml.MappingNode {
		return "", false, fmt.Errorf(
			"runner configuration root must be a mapping",
		)
	}

	for i := 0; i < len(root.Content); i += 2 {
		keyNode := root.Content[i]

		if keyNode.Kind != yaml.ScalarNode ||
			keyNode.Value != key {
			continue
		}

		valueNode := root.Content[i+1]

		if valueNode.Tag == "!!null" {
			return "", true, fmt.Errorf(
				"top-level %s must not be null",
				key,
			)
		}

		if valueNode.Kind != yaml.ScalarNode ||
			valueNode.Tag != "!!str" {
			return "", true, fmt.Errorf(
				"top-level %s must be a string",
				key,
			)
		}

		return valueNode.Value, true, nil
	}

	return "", false, nil
}
