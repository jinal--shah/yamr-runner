package config

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRunnerConfigExtractsRunnerOnly(
	t *testing.T,
) {
	document := parseRunnerYAML(
		t,
		`
yamr-sources-dir: /sources
yamr-source-labels: /labels.yaml

yamr-runner:
  env:
    ENTITY: m4m
    STACK: prod
`,
	)

	got, err := RunnerConfig(document)
	if err != nil {
		t.Fatalf(
			"RunnerConfig() error = %v",
			err,
		)
	}

	want := parseRunnerYAML(
		t,
		`
yamr-runner:
  env:
    ENTITY: m4m
    STACK: prod
`,
	)

	assertRunnerYAMLEqual(
		t,
		got,
		want,
	)
}

func TestRunnerConfigIgnoresOtherTopLevelKeys(
	t *testing.T,
) {
	document := parseRunnerYAML(
		t,
		`
something:
  nested:
    value: ignored

yamr-runner:
  env:
    ENTITY: m4m

another-key:
  - one
  - two
`,
	)

	got, err := RunnerConfig(document)
	if err != nil {
		t.Fatalf(
			"RunnerConfig() error = %v",
			err,
		)
	}

	want := parseRunnerYAML(
		t,
		`
yamr-runner:
  env:
    ENTITY: m4m
`,
	)

	assertRunnerYAMLEqual(
		t,
		got,
		want,
	)
}

func TestRunnerConfigAbsentReturnsEmptyDocument(
	t *testing.T,
) {
	document := parseRunnerYAML(
		t,
		`
yamr-sources-dir: /sources
yamr-source-labels: /labels.yaml
`,
	)

	got, err := RunnerConfig(document)
	if err != nil {
		t.Fatalf(
			"RunnerConfig() error = %v",
			err,
		)
	}

	root := got.Content[0]

	if root.Kind != yaml.MappingNode {
		t.Fatalf(
			"root kind = %v, want mapping",
			root.Kind,
		)
	}

	if len(root.Content) != 0 {
		t.Fatalf(
			"root content = %#v, want empty",
			root.Content,
		)
	}
}

func TestRunnerConfigNullReturnsEmptyDocument(
	t *testing.T,
) {
	document := parseRunnerYAML(
		t,
		`
yamr-runner: null
`,
	)

	got, err := RunnerConfig(document)
	if err != nil {
		t.Fatalf(
			"RunnerConfig() error = %v",
			err,
		)
	}

	if len(got.Content[0].Content) != 0 {
		t.Fatalf(
			"root is not empty: %#v",
			got.Content[0].Content,
		)
	}
}

func TestRunnerConfigRejectsNonMappingRunner(
	t *testing.T,
) {
	tests := []string{
		`yamr-runner: value`,
		`yamr-runner: [one, two]`,
	}

	for _, value := range tests {
		t.Run(
			value,
			func(t *testing.T) {
				document := parseRunnerYAML(
					t,
					value,
				)

				_, err := RunnerConfig(
					document,
				)
				if err == nil {
					t.Fatal(
						"RunnerConfig() succeeded, want error",
					)
				}

				if !strings.Contains(
					err.Error(),
					"must be a mapping",
				) {
					t.Fatalf(
						"error = %q",
						err,
					)
				}
			},
		)
	}
}

func TestRunnerConfigDoesNotMutateInput(
	t *testing.T,
) {
	document := parseRunnerYAML(
		t,
		`
yamr-sources-dir: /sources

yamr-runner:
  env:
    ENTITY: m4m
`,
	)

	before := cloneNode(document)

	got, err := RunnerConfig(document)
	if err != nil {
		t.Fatal(err)
	}

	// Mutate the extracted tree.
	gotRunner := got.Content[0].Content[1]
	gotRunner.Content = nil

	assertRunnerYAMLEqual(
		t,
		document,
		before,
	)
}

func parseRunnerYAML(
	t *testing.T,
	value string,
) *yaml.Node {
	t.Helper()

	var document yaml.Node

	if err := yaml.Unmarshal(
		[]byte(value),
		&document,
	); err != nil {
		t.Fatal(err)
	}

	return &document
}

func assertRunnerYAMLEqual(
	t *testing.T,
	got *yaml.Node,
	want *yaml.Node,
) {
	t.Helper()

	gotValue, err := runnerNodeValue(got)
	if err != nil {
		t.Fatal(err)
	}

	wantValue, err := runnerNodeValue(want)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(
		gotValue,
		wantValue,
	) {
		t.Fatalf(
			"YAML = %#v\nwant %#v",
			gotValue,
			wantValue,
		)
	}
}

func runnerNodeValue(
	node *yaml.Node,
) (any, error) {
	var value any

	if err := node.Decode(
		&value,
	); err != nil {
		return nil, err
	}

	return value, nil
}
