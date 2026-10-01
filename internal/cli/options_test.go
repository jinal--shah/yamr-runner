package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseUsesConfigValues(
	t *testing.T,
) {
	configFile := writeRunnerConfig(
		t,
		`
yamr-sources-dir: /config/sources
yamr-source-labels: /config/labels.yaml

yamr-runner:
  env:
    ENTITY: default
`,
	)

	got, err := Parse(
		[]string{
			"--config",
			configFile,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if got.ConfigFile != configFile {
		t.Fatalf(
			"ConfigFile = %q, want %q",
			got.ConfigFile,
			configFile,
		)
	}

	if got.YamrSourcesDir != "/config/sources" {
		t.Fatalf(
			"YamrSourcesDir = %q",
			got.YamrSourcesDir,
		)
	}

	if got.YamrSourceLabels !=
		"/config/labels.yaml" {
		t.Fatalf(
			"YamrSourceLabels = %q",
			got.YamrSourceLabels,
		)
	}
}

func TestParseEnvironmentOverridesConfig(
	t *testing.T,
) {
	configFile := writeRunnerConfig(
		t,
		`
yamr-sources-dir: /config/sources
yamr-source-labels: /config/labels.yaml
`,
	)

	got, err := Parse(
		[]string{
			"--config",
			configFile,
		},
		[]string{
			"YAMR_SOURCES_DIR=/env/sources",
			"YAMR_SOURCE_LABELS=/env/labels.yaml",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if got.YamrSourcesDir != "/env/sources" {
		t.Fatalf(
			"YamrSourcesDir = %q",
			got.YamrSourcesDir,
		)
	}

	if got.YamrSourceLabels !=
		"/env/labels.yaml" {
		t.Fatalf(
			"YamrSourceLabels = %q",
			got.YamrSourceLabels,
		)
	}
}

func TestParseCLIOverridesEnvironmentAndConfig(
	t *testing.T,
) {
	configFile := writeRunnerConfig(
		t,
		`
yamr-sources-dir: /config/sources
yamr-source-labels: /config/labels.yaml
`,
	)

	got, err := Parse(
		[]string{
			"--config",
			configFile,
			"--yamr-sources-dir",
			"/cli/sources",
			"--yamr-source-labels",
			"/cli/labels.yaml",
		},
		[]string{
			"YAMR_SOURCES_DIR=/env/sources",
			"YAMR_SOURCE_LABELS=/env/labels.yaml",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if got.YamrSourcesDir != "/cli/sources" {
		t.Fatalf(
			"YamrSourcesDir = %q",
			got.YamrSourcesDir,
		)
	}

	if got.YamrSourceLabels !=
		"/cli/labels.yaml" {
		t.Fatalf(
			"YamrSourceLabels = %q",
			got.YamrSourceLabels,
		)
	}
}

func TestParseConfigFromEnvironment(
	t *testing.T,
) {
	configFile := writeRunnerConfig(
		t,
		`
yamr-sources-dir: /sources
yamr-source-labels: /labels.yaml
`,
	)

	got, err := Parse(
		nil,
		[]string{
			"YAMR_RUNNER_CONFIG=" + configFile,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if got.ConfigFile != configFile {
		t.Fatalf(
			"ConfigFile = %q, want %q",
			got.ConfigFile,
			configFile,
		)
	}
}

func TestParseShortConfigOption(
	t *testing.T,
) {
	configFile := writeRunnerConfig(
		t,
		`
yamr-sources-dir: /sources
yamr-source-labels: /labels.yaml
`,
	)

	got, err := Parse(
		[]string{
			"-c",
			configFile,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if got.ConfigFile != configFile {
		t.Fatalf(
			"ConfigFile = %q, want %q",
			got.ConfigFile,
			configFile,
		)
	}
}

func TestParseErrors(
	t *testing.T,
) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) ([]string, []string)
		wantError string
	}{
		{
			name: "missing config",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				return nil, nil
			},
			wantError: "--config or YAMR_RUNNER_CONFIG is required",
		},
		{
			name: "empty config CLI value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				return []string{
					"--config=",
				}, nil
			},
			wantError: "--config must not be empty",
		},
		{
			name: "empty config environment value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				return nil, []string{
					"YAMR_RUNNER_CONFIG=",
				}
			},
			wantError: "YAMR_RUNNER_CONFIG must not be empty",
		},
		{
			name: "missing yamr sources dir",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-source-labels: /labels.yaml
`,
				)

				return []string{
					"--config",
					configFile,
				}, nil
			},
			wantError: "yamr-sources-dir is required",
		},
		{
			name: "missing yamr source labels",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: /sources
`,
				)

				return []string{
					"--config",
					configFile,
				}, nil
			},
			wantError: "yamr-source-labels is required",
		},

		// CLI is the winning source. An explicitly empty CLI
		// value must fail rather than falling through to env
		// or config.
		{
			name: "empty yamr sources dir CLI value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: /config/sources
yamr-source-labels: /labels.yaml
`,
				)

				return []string{
						"--config",
						configFile,
						"--yamr-sources-dir=",
					},
					[]string{
						"YAMR_SOURCES_DIR=/env/sources",
					}
			},
			wantError: "--yamr-sources-dir must not be empty",
		},
		{
			name: "empty yamr source labels CLI value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: /sources
yamr-source-labels: /config/labels.yaml
`,
				)

				return []string{
						"--config",
						configFile,
						"--yamr-source-labels=",
					},
					[]string{
						"YAMR_SOURCE_LABELS=/env/labels.yaml",
					}
			},
			wantError: "--yamr-source-labels must not be empty",
		},

		// Environment is the winning source. Empty means
		// invalid rather than "not configured".
		{
			name: "empty yamr sources dir environment value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: /config/sources
yamr-source-labels: /labels.yaml
`,
				)

				return []string{
						"--config",
						configFile,
					},
					[]string{
						"YAMR_SOURCES_DIR=",
					}
			},
			wantError: "YAMR_SOURCES_DIR must not be empty",
		},
		{
			name: "empty yamr source labels environment value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: /sources
yamr-source-labels: /config/labels.yaml
`,
				)

				return []string{
						"--config",
						configFile,
					},
					[]string{
						"YAMR_SOURCE_LABELS=",
					}
			},
			wantError: "YAMR_SOURCE_LABELS must not be empty",
		},

		// Config is the winning source.
		{
			name: "empty yamr sources dir config value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: ""
yamr-source-labels: /labels.yaml
`,
				)

				return []string{
					"--config",
					configFile,
				}, nil
			},
			wantError: "top-level yamr-sources-dir must not be empty",
		},
		{
			name: "empty yamr source labels config value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: /sources
yamr-source-labels: ""
`,
				)

				return []string{
					"--config",
					configFile,
				}, nil
			},
			wantError: "top-level yamr-source-labels must not be empty",
		},

		// Explicit YAML null is invalid for invocation
		// settings.
		{
			name: "null yamr sources dir config value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: null
yamr-source-labels: /labels.yaml
`,
				)

				return []string{
					"--config",
					configFile,
				}, nil
			},
			wantError: "top-level yamr-sources-dir must not be null",
		},
		{
			name: "null yamr source labels config value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: /sources
yamr-source-labels: null
`,
				)

				return []string{
					"--config",
					configFile,
				}, nil
			},
			wantError: "top-level yamr-source-labels must not be null",
		},

		// Invocation paths must be actual YAML strings.
		{
			name: "numeric yamr sources dir config value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: 123
yamr-source-labels: /labels.yaml
`,
				)

				return []string{
					"--config",
					configFile,
				}, nil
			},
			wantError: "top-level yamr-sources-dir must be a string",
		},
		{
			name: "boolean yamr source labels config value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: /sources
yamr-source-labels: true
`,
				)

				return []string{
					"--config",
					configFile,
				}, nil
			},
			wantError: "top-level yamr-source-labels must be a string",
		},
		{
			name: "mapping yamr sources dir config value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir:
  path: /sources
yamr-source-labels: /labels.yaml
`,
				)

				return []string{
					"--config",
					configFile,
				}, nil
			},
			wantError: "top-level yamr-sources-dir must be a string",
		},
		{
			name: "sequence yamr source labels config value",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: /sources
yamr-source-labels:
  - /one.yaml
  - /two.yaml
`,
				)

				return []string{
					"--config",
					configFile,
				}, nil
			},
			wantError: "top-level yamr-source-labels must be a string",
		},

		// flag.FlagSet itself rejects unknown options.
		{
			name: "unknown flag",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				return []string{
					"--does-not-exist",
				}, nil
			},
			wantError: "flag provided but not defined",
		},

		// yamr-runner currently accepts no positional
		// arguments.
		{
			name: "positional argument",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: /sources
yamr-source-labels: /labels.yaml
`,
				)

				return []string{
					"--config",
					configFile,
					"something",
				}, nil
			},
			wantError: "unexpected arguments: something",
		},
		{
			name: "multiple positional arguments",
			setup: func(
				t *testing.T,
			) ([]string, []string) {
				configFile := writeRunnerConfig(
					t,
					`
yamr-sources-dir: /sources
yamr-source-labels: /labels.yaml
`,
				)

				return []string{
					"--config",
					configFile,
					"one",
					"two",
				}, nil
			},
			wantError: "unexpected arguments: one two",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				args, environ := test.setup(t)

				_, err := Parse(
					args,
					environ,
				)
				if err == nil {
					t.Fatal(
						"Parse() succeeded, want error",
					)
				}

				if !strings.Contains(
					err.Error(),
					test.wantError,
				) {
					t.Fatalf(
						"Parse() error = %q, want error containing %q",
						err,
						test.wantError,
					)
				}
			},
		)
	}
}

func TestParseCLIIgnoresInvalidLowerPrioritySourcesDir(
	t *testing.T,
) {
	tests := []struct {
		name       string
		configYAML string
		environ    []string
	}{
		{
			name: "empty environment and null config",
			configYAML: `
yamr-sources-dir: null
yamr-source-labels: /labels.yaml
`,
			environ: []string{
				"YAMR_SOURCES_DIR=",
			},
		},
		{
			name: "environment and invalid numeric config",
			configYAML: `
yamr-sources-dir: 123
yamr-source-labels: /labels.yaml
`,
			environ: []string{
				"YAMR_SOURCES_DIR=/env/sources",
			},
		},
		{
			name: "empty environment and invalid mapping config",
			configYAML: `
yamr-sources-dir:
  path: /wrong
yamr-source-labels: /labels.yaml
`,
			environ: []string{
				"YAMR_SOURCES_DIR=",
			},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				configFile := writeRunnerConfig(
					t,
					test.configYAML,
				)

				got, err := Parse(
					[]string{
						"--config",
						configFile,
						"--yamr-sources-dir",
						"/cli/sources",
					},
					test.environ,
				)
				if err != nil {
					t.Fatalf(
						"Parse() error = %v",
						err,
					)
				}

				if got.YamrSourcesDir != "/cli/sources" {
					t.Fatalf(
						"YamrSourcesDir = %q, want %q",
						got.YamrSourcesDir,
						"/cli/sources",
					)
				}

				if got.YamrSourceLabels != "/labels.yaml" {
					t.Fatalf(
						"YamrSourceLabels = %q, want %q",
						got.YamrSourceLabels,
						"/labels.yaml",
					)
				}
			},
		)
	}
}

func TestParseEnvironmentIgnoresInvalidLowerPrioritySourcesDir(
	t *testing.T,
) {
	configFile := writeRunnerConfig(
		t,
		`
yamr-sources-dir: null
yamr-source-labels: /labels.yaml
`,
	)

	got, err := Parse(
		[]string{
			"--config",
			configFile,
		},
		[]string{
			"YAMR_SOURCES_DIR=/env/sources",
		},
	)
	if err != nil {
		t.Fatalf(
			"Parse() error = %v",
			err,
		)
	}

	if got.YamrSourcesDir != "/env/sources" {
		t.Fatalf(
			"YamrSourcesDir = %q, want %q",
			got.YamrSourcesDir,
			"/env/sources",
		)
	}
}

func TestParseNoPromptsDefaultsFalse(
	t *testing.T,
) {
	configFile := writeRunnerConfig(
		t,
		`
yamr-sources-dir: /sources
yamr-source-labels: /labels.yaml
`,
	)

	got, err := Parse(
		[]string{
			"--config",
			configFile,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if got.NoPrompts {
		t.Fatal(
			"NoPrompts = true, want false",
		)
	}
}

func TestParseNoPromptsFromCLI(
	t *testing.T,
) {
	configFile := writeRunnerConfig(
		t,
		`
yamr-sources-dir: /sources
yamr-source-labels: /labels.yaml
`,
	)

	got, err := Parse(
		[]string{
			"--config",
			configFile,
			"--no-prompts",
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !got.NoPrompts {
		t.Fatal(
			"NoPrompts = false, want true",
		)
	}
}

func TestParseIgnoresNoPromptsOutsideCLI(
	t *testing.T,
) {
	configFile := writeRunnerConfig(
		t,
		`
yamr-sources-dir: /sources
yamr-source-labels: /labels.yaml
no-prompts: true
`,
	)

	got, err := Parse(
		[]string{
			"--config",
			configFile,
		},
		[]string{
			"YAMR_RUNNER_NO_PROMPTS=true",
			"YAMR_NO_PROMPTS=true",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if got.NoPrompts {
		t.Fatal(
			"NoPrompts = true, want false; " +
				"no-prompts must only be enabled by CLI",
		)
	}
}

func TestParseMaxWorkers(
	t *testing.T,
) {
	configPath := writeRunnerConfig(
		t,
		`
yamr-sources-dir: /sources
yamr-source-labels: /labels.yaml
`,
	)

	tests := []struct {
		name    string
		args    []string
		environ []string
		want    int
		wantErr string
	}{
		{
			name: "default",
			want: DefaultMaxWorkers,
		},
		{
			name: "cli minimum",
			args: []string{
				"--max-workers",
				"1",
			},
			want: 1,
		},
		{
			name: "cli maximum",
			args: []string{
				"--max-workers",
				"8",
			},
			want: 8,
		},
		{
			name: "environment",
			environ: []string{
				"YAMR_RUNNER_MAX_WORKERS=6",
			},
			want: 6,
		},
		{
			name: "cli overrides environment",
			args: []string{
				"--max-workers",
				"3",
			},
			environ: []string{
				"YAMR_RUNNER_MAX_WORKERS=6",
			},
			want: 3,
		},
		{
			name: "invalid environment ignored when cli wins",
			args: []string{
				"--max-workers",
				"6",
			},
			environ: []string{
				"YAMR_RUNNER_MAX_WORKERS=garbage",
			},
			want: 6,
		},
		{
			name: "cli zero",
			args: []string{
				"--max-workers",
				"0",
			},
			wantErr: "--max-workers must be between 1 and 8",
		},
		{
			name: "cli negative",
			args: []string{
				"--max-workers",
				"-1",
			},
			wantErr: "--max-workers must be between 1 and 8",
		},
		{
			name: "cli above maximum",
			args: []string{
				"--max-workers",
				"9",
			},
			wantErr: "--max-workers must be between 1 and 8",
		},
		{
			name: "cli not integer",
			args: []string{
				"--max-workers",
				"garbage",
			},
			wantErr: `invalid value "garbage" for flag -max-workers: invalid integer "garbage"`,
		},
		{
			name: "environment empty",
			environ: []string{
				"YAMR_RUNNER_MAX_WORKERS=",
			},
			wantErr: "YAMR_RUNNER_MAX_WORKERS must not be empty",
		},
		{
			name: "environment zero",
			environ: []string{
				"YAMR_RUNNER_MAX_WORKERS=0",
			},
			wantErr: "envvar YAMR_RUNNER_MAX_WORKERS must be between 1 and 8",
		},
		{
			name: "environment negative",
			environ: []string{
				"YAMR_RUNNER_MAX_WORKERS=-1",
			},
			wantErr: "envvar YAMR_RUNNER_MAX_WORKERS must be between 1 and 8",
		},
		{
			name: "environment above maximum",
			environ: []string{
				"YAMR_RUNNER_MAX_WORKERS=9",
			},
			wantErr: "envvar YAMR_RUNNER_MAX_WORKERS must be between 1 and 8",
		},
		{
			name: "environment not integer",
			environ: []string{
				"YAMR_RUNNER_MAX_WORKERS=garbage",
			},
			wantErr: `YAMR_RUNNER_MAX_WORKERS must be an integer: "garbage"`,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				args := append(
					[]string{
						"--config",
						configPath,
					},
					test.args...,
				)

				options, err := Parse(
					args,
					test.environ,
				)

				if test.wantErr != "" {
					if err == nil {
						t.Fatalf(
							"Parse() error = nil, want %q",
							test.wantErr,
						)
					}

					if err.Error() != test.wantErr {
						t.Fatalf(
							"Parse() error = %q, want %q",
							err,
							test.wantErr,
						)
					}

					return
				}

				if err != nil {
					t.Fatalf(
						"Parse() error = %v",
						err,
					)
				}

				if options.MaxWorkers != test.want {
					t.Errorf(
						"MaxWorkers = %d, want %d",
						options.MaxWorkers,
						test.want,
					)
				}
			},
		)
	}
}

func TestParseMaxWorkersIgnoresConfig(
	t *testing.T,
) {
	configPath := writeRunnerConfig(
		t,
		`
yamr-sources-dir: /sources
yamr-source-labels: /labels.yaml
max-workers: 8
`,
	)

	options, err := Parse(
		[]string{
			"--config",
			configPath,
		},
		nil,
	)
	if err != nil {
		t.Fatalf(
			"Parse() error = %v",
			err,
		)
	}

	if options.MaxWorkers != DefaultMaxWorkers {
		t.Errorf(
			"MaxWorkers = %d, want default %d",
			options.MaxWorkers,
			DefaultMaxWorkers,
		)
	}
}

func writeRunnerConfig(
	t *testing.T,
	content string,
) string {
	t.Helper()

	path := filepath.Join(
		t.TempDir(),
		"yamr-runner.yaml",
	)

	if err := os.WriteFile(
		path,
		[]byte(content),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	return path
}
