package tokens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func helperCanonicalPath(t *testing.T, path string) string {
	t.Helper()

	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", path, err)
	}

	return path
}

func TestImmediateTokens(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, "run")
	thisDir := filepath.Join(root, "repo", "foo")

	for _, dir := range []string{runDir, thisDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	sourcesDir := filepath.Join(root, "sources")

	if err := os.MkdirAll(sourcesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	document := parseDocument(t, `
values:
  uid: "$uid_me$"
  gid: "$gid_me$"
  repo: "/$repo_root$"
  this: "/$this_dir$"
  relative: "$this_dir_rel_to_repo_root$"
  run_no_slashes: "$run_dir$"
  run_all_slashes: "/$run_dir$/"
  sources: "/$yamr_sources_dir$"
`)

	err := ResolveDocument(document, Context{
		ThisDir:        thisDir,
		RepoRoot:       filepath.Join(root, "repo"),
		RunDir:         runDir,
		YamrSourcesDir: sourcesDir,
	}, Immediate)

	if err != nil {
		t.Fatalf("ResolveDocument() error = %v", err)
	}

	values := mappingValue(t, document.Content[0], "values")

	assertScalar(t, values, "uid", strconvUID())
	assertScalar(t, values, "gid", strconvGID())
	assertScalar(t, values, "repo", helperCanonicalPath(t, filepath.Join(root, "repo")))
	assertScalar(t, values, "this", helperCanonicalPath(t, thisDir))
	assertScalar(t, values, "relative", "foo")
	assertScalar(t, values, "run_no_slashes", slashless(helperCanonicalPath(t, runDir)))
	assertScalar(t, values, "run_all_slashes", helperCanonicalPath(t, runDir) + "/")
	assertScalar(t, values, "sources", helperCanonicalPath(t, filepath.Join(root, "sources")))
}

func TestImmediateTokensFollowSymlinks(t *testing.T) {
	root := t.TempDir()

	realDir := filepath.Join(root, "real")
	linkDir := filepath.Join(root, "link")

	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatal(err)
	}

	document := parseDocument(t, `
path: "/$this_dir$"
`)

	err := ResolveDocument(document, Context{
		ThisDir: linkDir,
	}, Immediate)

	if err != nil {
		t.Fatalf("ResolveDocument() error = %v", err)
	}

	want := helperCanonicalPath(t, realDir)
	assertScalar(t, document.Content[0], "path", want)
}

func TestDeferredTokensRemainUnchangedDuringImmediateResolution(t *testing.T) {
	document := parseDocument(t, `
values:
  action: "$action_dir$/foo"
  env: "$env.FOO$"
  sources: "$yamr_sources$"
  labelled: "$yamr_sources_from_label$"
`)

	err := ResolveDocument(document, Context{
		ThisDir: t.TempDir(),
	}, Immediate)

	if err != nil {
		t.Fatalf("ResolveDocument() error = %v", err)
	}

	values := mappingValue(t, document.Content[0], "values")

	assertScalar(t, values, "action", "$action_dir$/foo")
	assertScalar(t, values, "env", "$env.FOO$")

	if got := mappingValue(t, values, "sources").Kind; got != yaml.ScalarNode {
		t.Fatalf("sources kind = %v, want ScalarNode", got)
	}

	if got := mappingValue(t, values, "labelled").Kind; got != yaml.ScalarNode {
		t.Fatalf("labelled kind = %v, want ScalarNode", got)
	}
}

func TestFinalActionDirToken(t *testing.T) {
	actionDir := filepath.Join(t.TempDir(), "action")

	if err := os.MkdirAll(actionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	document := parseDocument(t, `
path: "/$action_dir$/.generated"
`)

	err := ResolveDocument(document, Context{
		ActionDir: actionDir,
	}, Final)

	if err != nil {
		t.Fatalf("ResolveDocument() error = %v", err)
	}

	assertScalar(
		t,
		document.Content[0],
		"path",
		helperCanonicalPath(t, actionDir)+"/.generated",
	)
}

func TestFinalEnvUsesMergedYAMLEnvironment(t *testing.T) {
	document := parseDocument(t, `
values:
  yaml: "$env.YAML_VALUE$"
  host: "$env.HOST_VALUE$"
`)

	err := ResolveDocument(document, Context{
		Env: map[string]string{
			"YAML_VALUE": "from-yaml",
			"HOST_VALUE": "from-host",
		},
	}, Final)

	if err != nil {
		t.Fatalf("ResolveDocument() error = %v", err)
	}

	values := mappingValue(t, document.Content[0], "values")

	assertScalar(t, values, "yaml", "from-yaml")
	assertScalar(t, values, "host", "from-host")
}

func TestFinalEnvMissingVariableFails(t *testing.T) {
	document := parseDocument(t, `
value: "$env.DOES_NOT_EXIST$"
`)

	err := ResolveDocument(document, Context{
		Env: map[string]string{},
	}, Final)

	if err == nil {
		t.Fatal("ResolveDocument() succeeded, want error")
	}

	if !strings.Contains(err.Error(), `DOES_NOT_EXIST`) {
		t.Fatalf("error = %q, want variable name", err)
	}
}

func TestFinalYamrSourcesBecomesSequence(t *testing.T) {
	document := parseDocument(t, `
cmd_sources: $yamr_sources$
`)

	err := ResolveDocument(document, Context{
		YamrSources: []string{
			"one.yaml",
			"two.yaml",
		},
	}, Final)

	if err != nil {
		t.Fatalf("ResolveDocument() error = %v", err)
	}

	cmdSources := mappingValue(t, document.Content[0], "cmd_sources")

	if cmdSources.Kind != yaml.SequenceNode {
		t.Fatalf("cmdSources kind = %v, want SequenceNode", cmdSources.Kind)
	}

	assertSequence(t, cmdSources, []string{
		"one.yaml",
		"two.yaml",
	})
}

func TestFinalYamrSourcesFromLabelBecomesSequence(t *testing.T) {
	document := parseDocument(t, `
cmd_sources: $yamr_sources_from_label$
`)

	err := ResolveDocument(document, Context{
		YamrSourcesFromLabel: []string{
			"foo.yaml",
			"bar.yaml",
		},
	}, Final)

	if err != nil {
		t.Fatalf("ResolveDocument() error = %v", err)
	}

	cmdSources := mappingValue(t, document.Content[0], "cmd_sources")

	if cmdSources.Kind != yaml.SequenceNode {
		t.Fatalf("cmdSources kind = %v, want SequenceNode", cmdSources.Kind)
	}

	assertSequence(t, cmdSources, []string{
		"foo.yaml",
		"bar.yaml",
	})
}

func TestSourceTokenMustBeWholeScalar(t *testing.T) {
	tokens := []string{
		"$yamr_sources$",
		"$yamr_sources_from_label$",
	}

	values := []string{
		"prefix-%s",
		"%s-suffix",
		"prefix-%s-suffix",
	}

	for _, token := range tokens {
		for _, value := range values {
			t.Run(token+"/"+value, func(t *testing.T) {
				document := parseDocument(
					t,
					fmt.Sprintf(
						"cmd_sources: %q\n",
						fmt.Sprintf(value, token),
					),
				)

				err := ResolveDocument(
					document,
					Context{},
					Immediate,
				)

				if err == nil {
					t.Fatal("ResolveDocument() succeeded, want error")
				}

				if !strings.Contains(
					err.Error(),
					"must be the entire YAML value",
				) {
					t.Fatalf(
						"error = %q, want whole-value error",
						err,
					)
				}
			})
		}
	}
}

func TestUnknownTokenFails(t *testing.T) {
	document := parseDocument(t, `
value: "$this_is_not_a_token$"
`)

	err := ResolveDocument(document, Context{}, Immediate)
	if err == nil {
		t.Fatal("ResolveDocument() succeeded, want error")
	}
}

func TestMultipleScalarTokens(t *testing.T) {
	root := t.TempDir()

	repoRoot := filepath.Join(root, "repo")
	actionDir := filepath.Join(root, "action")

	for _, dir := range []string{
		repoRoot,
		actionDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", dir, err)
		}
	}

	document := parseDocument(t, `
path: "/$repo_root$/$env.NAME$/$action_dir$"
`)

	err := ResolveDocument(document, Context{
		RepoRoot:  repoRoot,
		ActionDir: actionDir,
		Env: map[string]string{
			"NAME": "value",
		},
	}, Final)

	if err != nil {
		t.Fatalf("ResolveDocument() error = %v", err)
	}

	path := mappingValue(t, document.Content[0], "path")

	canonicalRepoRoot := helperCanonicalPath(t, repoRoot)
	canonicalActionDir := helperCanonicalPath(t, actionDir)

	want := fmt.Sprintf(
		"/%s/value/%s",
		strings.Trim(canonicalRepoRoot, "/"),
		strings.Trim(canonicalActionDir, "/"),
	)
	if path.Value != want {
		t.Fatalf(
			"path = %q, want %q",
			path.Value,
			want,
		)
	}
}

func TestBuildEnvironmentYAMLOverridesHost(t *testing.T) {
	document := parseDocument(t, `
yamr-runner:
  env:
    ENTITY: m4m
`)

	got, err := BuildEnvironment(
		document,
		[]string{
			"ENTITY=from-host",
			"HOST_ONLY=host-value",
		},
	)

	if err != nil {
		t.Fatalf("BuildEnvironment() error = %v", err)
	}

	if got["ENTITY"] != "m4m" {
		t.Fatalf(
			"ENTITY = %q, want %q",
			got["ENTITY"],
			"m4m",
		)
	}

	if got["HOST_ONLY"] != "host-value" {
		t.Fatalf(
			"HOST_ONLY = %q, want %q",
			got["HOST_ONLY"],
			"host-value",
		)
	}
}

func TestBuildEnvironmentNullRemovesHostValue(t *testing.T) {
	document := parseDocument(t, `
yamr-runner:
  env:
    AWS_PROFILE: null
`)

	got, err := BuildEnvironment(
		document,
		[]string{
			"AWS_PROFILE=my-profile",
			"OTHER=value",
		},
	)

	if err != nil {
		t.Fatalf("BuildEnvironment() error = %v", err)
	}

	if _, exists := got["AWS_PROFILE"]; exists {
		t.Fatalf(
			"AWS_PROFILE exists with value %q, want absent",
			got["AWS_PROFILE"],
		)
	}

	if got["OTHER"] != "value" {
		t.Fatalf(
			"OTHER = %q, want %q",
			got["OTHER"],
			"value",
		)
	}
}

func TestBuildEnvironmentEmptyStringOverridesHost(t *testing.T) {
	document := parseDocument(t, `
yamr-runner:
  env:
    EMPTY: ""
`)

	got, err := BuildEnvironment(
		document,
		[]string{
			"EMPTY=host-value",
		},
	)

	if err != nil {
		t.Fatalf("BuildEnvironment() error = %v", err)
	}

	value, exists := got["EMPTY"]
	if !exists {
		t.Fatal("EMPTY is absent, want present")
	}

	if value != "" {
		t.Fatalf(
			"EMPTY = %q, want empty string",
			value,
		)
	}
}

func TestBuildEnvironmentStringifiesScalarLexicalValues(t *testing.T) {
	document := parseDocument(t, `
yamr-runner:
  env:
    STRING: hello
    INTEGER: 123
    FLOAT: 10.50
    TRUE: true
    FALSE: false
`)

	got, err := BuildEnvironment(document, nil)
	if err != nil {
		t.Fatalf("BuildEnvironment() error = %v", err)
	}

	want := map[string]string{
		"STRING":  "hello",
		"INTEGER": "123",
		"FLOAT":   "10.50",
		"TRUE":    "true",
		"FALSE":   "false",
	}

	for key, expected := range want {
		if got[key] != expected {
			t.Errorf(
				"%s = %q, want %q",
				key,
				got[key],
				expected,
			)
		}
	}
}

func TestBuildEnvironmentRejectsNonScalarValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name: "sequence",
			value: `
yamr-runner:
  env:
    FOO:
      - one
      - two
`,
		},
		{
			name: "mapping",
			value: `
yamr-runner:
  env:
    FOO:
      nested: value
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := parseDocument(t, tt.value)

			_, err := BuildEnvironment(document, nil)
			if err == nil {
				t.Fatal(
					"BuildEnvironment() succeeded, want error",
				)
			}
		})
	}
}

func TestBuildEnvironmentRejectsNonMappingEnv(t *testing.T) {
	document := parseDocument(t, `
yamr-runner:
  env:
    - foo
    - bar
`)

	_, err := BuildEnvironment(document, nil)
	if err == nil {
		t.Fatal("BuildEnvironment() succeeded, want error")
	}
}

func TestFinalEnvEmptyValueIsValid(t *testing.T) {
	document := parseDocument(t, `
value: "prefix-$env.EMPTY$-suffix"
`)

	err := ResolveDocument(
		document,
		Context{
			Env: map[string]string{
				"EMPTY": "",
			},
		},
		Final,
	)

	if err != nil {
		t.Fatalf("ResolveDocument() error = %v", err)
	}

	assertScalar(
		t,
		document.Content[0],
		"value",
		"prefix--suffix",
	)
}

func TestNullYAMLEnvCausesEnvTokenToBeUndefined(t *testing.T) {
	config := parseDocument(t, `
yamr-runner:
  env:
    FOO: null

value: "$env.FOO$"
`)

	env, err := BuildEnvironment(
		config,
		[]string{
			"FOO=from-host",
		},
	)
	if err != nil {
		t.Fatalf("BuildEnvironment() error = %v", err)
	}

	err = ResolveDocument(
		config,
		Context{
			Env: env,
		},
		Final,
	)

	if err == nil {
		t.Fatal("ResolveDocument() succeeded, want undefined env error")
	}

	if !strings.Contains(err.Error(), "FOO") {
		t.Fatalf(
			"error = %q, want reference to FOO",
			err,
		)
	}
}

func parseDocument(t *testing.T, source string) *yaml.Node {
	t.Helper()

	var document yaml.Node

	if err := yaml.Unmarshal([]byte(source), &document); err != nil {
		t.Fatalf("parsing test YAML: %v", err)
	}

	return &document
}

func mappingValue(t *testing.T, mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}

	t.Fatalf("mapping key %q not found", key)
	return nil
}

func assertScalar(t *testing.T, mapping *yaml.Node, key, want string) {
	t.Helper()

	value := mappingValue(t, mapping, key)

	if value.Kind != yaml.ScalarNode {
		t.Fatalf("%s kind = %v, want ScalarNode", key, value.Kind)
	}

	if value.Value != want {
		t.Fatalf("%s = %q, want %q", key, value.Value, want)
	}
}

func assertSequence(t *testing.T, sequence *yaml.Node, want []string) {
	t.Helper()

	if len(sequence.Content) != len(want) {
		t.Fatalf(
			"sequence length = %d, want %d",
			len(sequence.Content),
			len(want),
		)
	}

	for i, expected := range want {
		if sequence.Content[i].Value != expected {
			t.Errorf(
				"sequence[%d] = %q, want %q",
				i,
				sequence.Content[i].Value,
				expected,
			)
		}
	}
}

func slashless(path string) string {
	return strings.Trim(filepath.ToSlash(path), "/")
}

func strconvUID() string {
	return fmt.Sprintf("%d", os.Getuid())
}

func strconvGID() string {
	return fmt.Sprintf("%d", os.Getgid())
}
