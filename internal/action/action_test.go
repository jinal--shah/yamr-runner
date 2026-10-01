package action

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"jinal--shah/yamr-run/internal/discover"
	"jinal--shah/yamr-run/internal/sources"
)

func TestCompileRequiresSources(t *testing.T) {
	candidate := candidateFromYAML(t, `
yamr-runner:
  env:
    ENTITY: m4m
`)

	_, err := Compile(Options{
		Candidate: candidate,
	})

	if err == nil {
		t.Fatal("Compile() succeeded, want error")
	}

	if !strings.Contains(
		err.Error(),
		"sources",
	) {
		t.Fatalf("error = %q, want sources error", err)
	}
}

func TestCompileSourcesLabel(t *testing.T) {
	labels := loadLabels(t, `
vpc-platform:
  - aws_account/$env.ENTITY$.yaml
  - platform/$env.PLATFORM$/common.yaml
`)

	candidate := candidateFromYAML(t, `
yamr-runner:
  sources-label: vpc-platform

  env:
    ENTITY: m4m
    PLATFORM: polaris

  action:
    run:
      docker:
        cmd_sources: $yamr_sources_from_label$
`)

	action, err := Compile(Options{
		Candidate: candidate,
		Labels:    labels,
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	want := []string{
		"aws_account/m4m.yaml",
		"platform/polaris/common.yaml",
	}

	assertStringSlice(t, action.Sources, want)
}

func TestCompileSourceTokensCanUseHostEnvironment(t *testing.T) {
	labels := loadLabels(t, `
example:
  - $env.HOST_VALUE$.yaml
`)

	candidate := candidateFromYAML(t, `
yamr-runner:
  sources-label: example

  action:
    run:
      docker:
        cmd_sources: $yamr_sources_from_label$
`)

	action, err := Compile(Options{
		Candidate: candidate,
		Labels:    labels,
		HostEnv: []string{
			"HOST_VALUE=from-host",
		},
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	assertStringSlice(
		t,
		action.Sources,
		[]string{"from-host.yaml"},
	)
}

func TestCompileNullEnvSuppressesHostFallback(t *testing.T) {
	labels := loadLabels(t, `
example:
  - $env.ENTITY$.yaml
`)

	candidate := candidateFromYAML(t, `
yamr-runner:
  sources-label: example

  env:
    ENTITY: null

  action:
    run:
      docker:
        cmd_sources: $yamr_sources_from_label$
`)

	_, err := Compile(Options{
		Candidate: candidate,
		Labels:    labels,
		HostEnv: []string{
			"ENTITY=from-host",
		},
	})

	if err == nil {
		t.Fatal("Compile() succeeded, want error")
	}

	if !strings.Contains(err.Error(), "ENTITY") {
		t.Fatalf(
			"error = %q, want ENTITY error",
			err,
		)
	}
}

func TestCompileInlineSources(t *testing.T) {
	candidate := candidateFromYAML(t, `
yamr-runner:
  sources:
    - foo.yaml
    - platform/$env.PLATFORM$.yaml

  env:
    PLATFORM: polaris

  action:
    run:
      docker:
        cmd_sources: $yamr_sources$
`)

	action, err := Compile(Options{
		Candidate: candidate,
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	assertStringSlice(
		t,
		action.Sources,
		[]string{
			"foo.yaml",
			"platform/polaris.yaml",
		},
	)
}

func TestCompileInlineSourcesWinOverLabel(t *testing.T) {
	labels := loadLabels(t, `
label:
  - from-label.yaml
`)

	candidate := candidateFromYAML(t, `
yamr-runner:
  sources:
    - inline.yaml

  sources-label: label

  action:
    run:
      docker:
        cmd_sources: $yamr_sources$
`)

	action, err := Compile(Options{
		Candidate: candidate,
		Labels:    labels,
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	assertStringSlice(
		t,
		action.Sources,
		[]string{"inline.yaml"},
	)
}

func TestCompileInlineSourcesWinOverLabelAndRequireInlineToken(
	t *testing.T,
) {
	labels := loadLabels(t, `
fallback:
  - from-label.yaml
`)

	candidate := candidateFromYAML(t, `
yamr-runner:
  sources:
    - inline.yaml

  sources-label: fallback

  action:
    run:
      docker:
        cmd_sources: $yamr_sources_from_label$
`)

	_, err := Compile(Options{
		Candidate: candidate,
		Labels:    labels,
	})

	if err == nil {
		t.Fatal("Compile() succeeded, want error")
	}

	want := "cmd_sources must be $yamr_sources$"

	if !strings.Contains(err.Error(), want) {
		t.Fatalf(
			"error = %q, want error containing %q",
			err,
			want,
		)
	}
}

func TestCompileInlineSourcesWrongCmdWithoutLabelFails(
	t *testing.T,
) {
	candidate := candidateFromYAML(t, `
yamr-runner:
  sources:
    - inline.yaml

  action:
    run:
      docker:
        cmd_sources: something-else
`)

	_, err := Compile(Options{
		Candidate: candidate,
	})

	if err == nil {
		t.Fatal("Compile() succeeded, want error")
	}

	if !strings.Contains(
		err.Error(),
		"$yamr_sources$",
	) {
		t.Fatalf(
			"error = %q, want yamr_sources guidance",
			err,
		)
	}
}

func TestCompileRejectsEmptyInlineSources(t *testing.T) {
	candidate := candidateFromYAML(t, `
yamr-runner:
  sources: []

  action:
    run:
      docker:
        cmd_sources: $yamr_sources$
`)

	_, err := Compile(Options{
		Candidate: candidate,
	})

	if err == nil {
		t.Fatal("Compile() succeeded, want error")
	}

	want := "selected yamr source list must not be empty"

	if !strings.Contains(err.Error(), want) {
		t.Fatalf(
			"error = %q, want error containing %q",
			err,
			want,
		)
	}
}

func TestCompileRejectsEmptySourcesLabel(t *testing.T) {
	labels := loadLabels(t, `
empty-label: []
`)

	candidate := candidateFromYAML(t, `
yamr-runner:
  sources-label: empty-label

  action:
    run:
      docker:
        cmd_sources: $yamr_sources_from_label$
`)

	_, err := Compile(Options{
		Candidate: candidate,
		Labels:    labels,
	})

	if err == nil {
		t.Fatal("Compile() succeeded, want error")
	}

	want := "selected yamr source list must not be empty"

	if !strings.Contains(err.Error(), want) {
		t.Fatalf(
			"error = %q, want error containing %q",
			err,
			want,
		)
	}
}

func TestCompileSourcesLabelRequiresLabelToken(t *testing.T) {
	labels := loadLabels(t, `
example:
  - foo.yaml
`)

	candidate := candidateFromYAML(t, `
yamr-runner:
  sources-label: example

  action:
    run:
      docker:
        cmd_sources: $yamr_sources$
`)

	_, err := Compile(Options{
		Candidate: candidate,
		Labels:    labels,
	})

	if err == nil {
		t.Fatal("Compile() succeeded, want error")
	}

	want := "cmd_sources must be $yamr_sources_from_label$"

	if !strings.Contains(err.Error(), want) {
		t.Fatalf(
			"error = %q, want error containing %q",
			err,
			want,
		)
	}
}

func TestCompileDockerEnvExcludesHostOnlyVariables(t *testing.T) {
	candidate := candidateFromYAML(t, `
yamr-runner:
  sources:
    - $env.HOST_ONLY$.yaml

  env:
    YAML_ONLY: configured

  action:
    run:
      docker:
        cmd_sources: $yamr_sources$
`)

	action, err := Compile(Options{
		Candidate: candidate,
		HostEnv: []string{
			"HOST_ONLY=host",
			"SECRET_HOST_VALUE=do-not-pass",
		},
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if action.Env["YAML_ONLY"] != "configured" {
		t.Fatalf(
			"YAML_ONLY = %q, want configured",
			action.Env["YAML_ONLY"],
		)
	}

	if _, exists := action.Env["HOST_ONLY"]; exists {
		t.Fatal("HOST_ONLY unexpectedly present in Docker env")
	}

	if _, exists := action.Env["SECRET_HOST_VALUE"]; exists {
		t.Fatal(
			"SECRET_HOST_VALUE unexpectedly present in Docker env",
		)
	}

	assertStringSlice(
		t,
		action.Sources,
		[]string{"host.yaml"},
	)
}

// TODO: add actual verifications - need nestedHelper mapping helper
func TestCompileResolvesFinalTokens(t *testing.T) {
	candidate := candidateFromYAML(t, `
yamr-runner:
  sources:
    - foo.yaml

  env:
    ENTITY: m4m

  action:
    pre_run:
      - mkdir /$action_dir$/.generated

    run:
      docker:
        cmd_sources: $yamr_sources$
        work_dir: /work/$env.ENTITY$
`)

	action, err := Compile(Options{
		Candidate: candidate,
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// The easiest assertion here is against the yaml.Node tree using
	// your existing nestedMapping/mapping helper functions.
	//
	// Verify:
	//
	// pre_run[0] == "mkdir /<canonical action dir>/.generated"
	// work_dir   == "/work/m4m"
	// cmd_sources == sequence ["foo.yaml"]
	_ = action
}

func TestCompileResolvesDeferredTokensInConfiguredEnvironment(
	t *testing.T,
) {
	candidate := candidateFromYAML(t, `
yamr-runner:
  sources:
    - source.yaml

  env:
    OUTPUT_DIR: /$action_dir$/.generated

  action:
    run:
      docker:
        cmd_sources: $yamr_sources$
`)

	action, err := Compile(Options{
		Candidate: candidate,
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if action == nil {
		t.Fatal("Compile() returned nil Action")
	}

	want := candidate.ActionDir + "/.generated"

	got, ok := action.Env["OUTPUT_DIR"]
	if !ok {
		t.Fatal(
			"Action.Env does not contain OUTPUT_DIR",
		)
	}

	if got != want {
		t.Fatalf(
			"Action.Env[OUTPUT_DIR] = %q, want %q",
			got,
			want,
		)
	}

	runner, err := runnerConfig(action.Config)
	if err != nil {
		t.Fatalf("runnerConfig() error = %v", err)
	}

	env := mappingValue(runner, "env")
	if env == nil {
		t.Fatal("final config has no yamr-runner.env")
	}

	outputDir := mappingValue(env, "OUTPUT_DIR")
	if outputDir == nil {
		t.Fatal(
			"final config has no yamr-runner.env.OUTPUT_DIR",
		)
	}

	if outputDir.Value != want {
		t.Fatalf(
			"final config OUTPUT_DIR = %q, want %q",
			outputDir.Value,
			want,
		)
	}
}

func parseYAML(t *testing.T, value string) *yaml.Node {
	t.Helper()

	var document yaml.Node

	if err := yaml.Unmarshal(
		[]byte(value),
		&document,
	); err != nil {
		t.Fatalf(
			"yaml.Unmarshal() error = %v",
			err,
		)
	}

	return &document
}

func canonicalTestPath(t *testing.T, path string) string {
	t.Helper()

	path, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf(
			"filepath.Abs(%q): %v",
			path,
			err,
		)
	}

	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf(
			"filepath.EvalSymlinks(%q): %v",
			path,
			err,
		)
	}

	return filepath.Clean(path)
}

func assertStringSlice(
	t *testing.T,
	got []string,
	want []string,
) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf(
			"got %q, want %q",
			got,
			want,
		)
	}
}

func candidateFromYAML(
	t *testing.T,
	value string,
) discover.Candidate {
	t.Helper()

	dir := canonicalTestPath(t, t.TempDir())

	return discover.Candidate{
		ActionFile: filepath.Join(dir, ".yamr.yaml"),
		ActionDir:  dir,
		Config:     parseYAML(t, value),
	}
}

func loadLabels(
	t *testing.T,
	value string,
) *sources.Labels {
	t.Helper()

	path := filepath.Join(
		t.TempDir(),
		"labels.yaml",
	)

	if err := os.WriteFile(
		path,
		[]byte(value),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	labels, err := sources.LoadLabels(path)
	if err != nil {
		t.Fatalf("LoadLabels() error = %v", err)
	}

	return labels
}
