package cli

import (
	"fmt"
	"strings"

	"jinal--shah/yamr-runner/internal/buildinfo"
)

type HelpRequest struct {
	Requested bool
	Topic     string
}

func ParseHelp(
	args []string,
) (HelpRequest, error) {
	for i, arg := range args {
		if arg != "-h" && arg != "--help" {
			continue
		}

		request := HelpRequest{
			Requested: true,
		}

		if i+1 < len(args) &&
			!strings.HasPrefix(args[i+1], "-") {
			request.Topic = args[i+1]
		}

		return request, nil
	}

	return HelpRequest{}, nil
}

const (
	HelpTopicExamples   = "examples"
	HelpTopicConfigFile = "config-file"
	HelpTopicActionFile = "action-file"
	HelpTopicOverrides  = "overrides"
	HelpTopicTokens     = "tokens"
)

func normalizeHelpTopic(
	topic string,
) string {
	return strings.ReplaceAll(
		topic,
		"_",
		"-",
	)
}

func Help(
	topic string,
) (string, error) {
	switch normalizeHelpTopic(topic) {
	case "":
		return buildinfo.BuildInfo() + generalHelp + "Topics\n" + helpTopicList, nil

	case HelpTopicExamples:
		return examplesHelp, nil

	case HelpTopicConfigFile:
		return configFileHelp, nil

	case HelpTopicActionFile:
		return actionFileHelp, nil

	case HelpTopicOverrides:
		return overridesHelp, nil

	case HelpTopicTokens:
		return tokensHelp, nil

	default:
		return "", fmt.Errorf(
			"unknown help topic %q\nValid Topics:\n%s",
			topic,
			helpTopicList,
		)
	}
}
