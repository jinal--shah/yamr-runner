package cli

const generalHelp = `yamr-runner

Discover triggered YAMR action files beneath the current directory and
execute their configured YAMR actions using Docker.

Usage:
  yamr-runner [options]
  yamr-runner -h [topic]
  yamr-runner --help [topic]

Options:

  -c, --config FILE
      Path to the yamr-runner configuration file.

      Required.

      May also be set with:
        YAMR_RUNNER_CONFIG

  --yamr-sources-dir DIR
      Directory containing the YAMR source files.

      Required.

      Resolution precedence:
        command line
        YAMR_SOURCES_DIR
        yamr-sources-dir in the configuration file

  --yamr-source-labels FILE
      YAML file mapping source labels to YAMR source files.

      Required.

      Resolution precedence:
        command line
        YAMR_SOURCE_LABELS
        yamr-source-labels in the configuration file

  --max-workers N
      Maximum number of concurrent Docker operations/actions.

      Default:
        4

      Allowed range:
        1..8

      May also be set with:
        YAMR_RUNNER_MAX_WORKERS

      This option is intentionally not read from the configuration file.
	  Set it to a value your hardware can handle.

  --no-prompts
      Do not request interactive confirmation.

      This is a command-line-only option.

  -h, --help [topic]
      Show help.

      With no topic, show this help.

`

const helpTopicList = `  yamr-runner -h examples    # Complete example invocation and supporting YAML files.
  yamr-runner -h config-file # Annotated yamr-runner configuration file.
  yamr-runner -h action-file # Annotated YAMR action file.
  yamr-runner -h overrides   # How yaml gets merged to create a docker run command
  yamr-runner -h tokens      # The $token$ syntax explained
`
