package discover

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"jinal--shah/yamr-run/internal/config"
)

func TestFindDoesNotTriggerByDefault(
	t *testing.T,
) {
	root := t.TempDir()

	writeTestFile(
		t,
		filepath.Join(
			root,
			".yamr.yaml",
		),
		`
yamr-runner:
  env:
    ENTITY: m4m
`,
	)

	result, err := Find(
		Options{
			RunDir:     root,
			RepoRoot:   root,
			ActionFile: ".yamr.yaml",
		},
	)
	if err != nil {
		t.Fatalf(
			"Find() error = %v",
			err,
		)
	}

	if len(result.Candidates) != 0 {
		t.Fatalf(
			"candidates = %d, want 0",
			len(result.Candidates),
		)
	}

	if len(result.IgnoredActionFiles) != 0 {
		t.Fatalf(
			"ignored action files = %d, want 0",
			len(result.IgnoredActionFiles),
		)
	}
}

func TestFindTriggerFalseDoesNotCreateCandidate(
	t *testing.T,
) {
	root := t.TempDir()

	writeTestFile(
		t,
		filepath.Join(
			root,
			".yamr.yaml",
		),
		`
trigger-yamr-runner: false

yamr-runner:
  env:
    ENTITY: m4m
`,
	)

	result, err := Find(
		Options{
			RunDir:     root,
			RepoRoot:   root,
			ActionFile: ".yamr.yaml",
		},
	)
	if err != nil {
		t.Fatalf(
			"Find() error = %v",
			err,
		)
	}

	if len(result.Candidates) != 0 {
		t.Fatalf(
			"candidates = %d, want 0",
			len(result.Candidates),
		)
	}
}

func TestFindTriggeredActionInheritsAncestorConfig(
	t *testing.T,
) {
	root := t.TempDir()

	polarisDir := filepath.Join(
		root,
		"polaris",
	)

	prodDir := filepath.Join(
		polarisDir,
		"prod",
	)

	if err := os.MkdirAll(
		prodDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	writeTestFile(
		t,
		filepath.Join(
			root,
			".yamr.yaml",
		),
		`
yamr-runner:
  env:
    ENTITY: m4m
    SHARED: root
`,
	)

	writeTestFile(
		t,
		filepath.Join(
			polarisDir,
			".yamr.yaml",
		),
		`
yamr-runner:
  env:
    PLATFORM: polaris
    SHARED: polaris
`,
	)

	writeTestFile(
		t,
		filepath.Join(
			prodDir,
			".yamr.yaml",
		),
		`
trigger-yamr-runner: true

yamr-runner:
  sources-label: prod
  env:
    STACK: prod
`,
	)

	result, err := Find(
		Options{
			RunDir:     root,
			RepoRoot:   root,
			ActionFile: ".yamr.yaml",
		},
	)
	if err != nil {
		t.Fatalf(
			"Find() error = %v",
			err,
		)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf(
			"candidates = %d, want 1",
			len(result.Candidates),
		)
	}

	candidate := result.Candidates[0]

	wantActionFile := canonicalTestPath(
		t,
		filepath.Join(
			prodDir,
			".yamr.yaml",
		),
	)

	if candidate.ActionFile != wantActionFile {
		t.Fatalf(
			"ActionFile = %q, want %q",
			candidate.ActionFile,
			wantActionFile,
		)
	}

	wantActionDir := canonicalTestPath(
		t,
		prodDir,
	)

	if candidate.ActionDir != wantActionDir {
		t.Fatalf(
			"ActionDir = %q, want %q",
			candidate.ActionDir,
			wantActionDir,
		)
	}

	var decoded struct {
		YamrRunner struct {
			SourcesLabel string `yaml:"sources-label"`

			Env map[string]string `yaml:"env"`
		} `yaml:"yamr-runner"`
	}

	if err := candidate.Config.Decode(
		&decoded,
	); err != nil {
		t.Fatalf(
			"Decode() error = %v",
			err,
		)
	}

	if decoded.YamrRunner.SourcesLabel != "prod" {
		t.Fatalf(
			"sources-label = %q, want prod",
			decoded.YamrRunner.SourcesLabel,
		)
	}

	wantEnv := map[string]string{
		"ENTITY":   "m4m",
		"PLATFORM": "polaris",
		"SHARED":   "polaris",
		"STACK":    "prod",
	}

	if !reflect.DeepEqual(
		decoded.YamrRunner.Env,
		wantEnv,
	) {
		t.Fatalf(
			"env = %#v, want %#v",
			decoded.YamrRunner.Env,
			wantEnv,
		)
	}

	if len(result.IgnoredActionFiles) != 0 {
		t.Fatalf(
			"ignored action files = %#v, want none",
			result.IgnoredActionFiles,
		)
	}
}

func TestFindTriggeredActionStopsDiscoveryBelowIt(
	t *testing.T,
) {
	root := t.TempDir()

	actionDir := filepath.Join(
		root,
		"A",
		"001",
	)

	yyyDir := filepath.Join(
		actionDir,
		"yyy",
	)

	zzzDir := filepath.Join(
		actionDir,
		"zzz",
	)

	if err := os.MkdirAll(
		yyyDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(
		zzzDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	triggerFile := filepath.Join(
		actionDir,
		".yamr.yaml",
	)

	writeTestFile(
		t,
		triggerFile,
		`
trigger-yamr-runner: true

yamr-runner:
  sources-label: primary
`,
	)

	//
	// This is valid YAML, but would be a trigger if normal
	// discovery reached it. It must instead be ignored.
	//
	yyyFile := filepath.Join(
		yyyDir,
		".yamr.yaml",
	)

	writeTestFile(
		t,
		yyyFile,
		`
trigger-yamr-runner: true

yamr-runner:
  sources-label: should-not-run
`,
	)

	//
	// Deliberately malformed YAML.
	//
	// This proves descendants are only scanned by filename and
	// are not parsed once an ancestor has triggered.
	//
	zzzFile := filepath.Join(
		zzzDir,
		".yamr.yaml",
	)

	writeTestFile(
		t,
		zzzFile,
		`
this is: [
`,
	)

	result, err := Find(
		Options{
			RunDir:     root,
			RepoRoot:   root,
			ActionFile: ".yamr.yaml",
		},
	)
	if err != nil {
		t.Fatalf(
			"Find() error = %v",
			err,
		)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf(
			"candidates = %d, want 1",
			len(result.Candidates),
		)
	}

	wantTrigger := canonicalTestPath(
		t,
		triggerFile,
	)

	if result.Candidates[0].ActionFile != wantTrigger {
		t.Fatalf(
			"candidate = %q, want %q",
			result.Candidates[0].ActionFile,
			wantTrigger,
		)
	}

	wantIgnored := []IgnoredActionFile{
		{
			ActionFile: canonicalTestPath(
				t,
				yyyFile,
			),
			TriggeredBy: wantTrigger,
		},
		{
			ActionFile: canonicalTestPath(
				t,
				zzzFile,
			),
			TriggeredBy: wantTrigger,
		},
	}

	if !reflect.DeepEqual(
		result.IgnoredActionFiles,
		wantIgnored,
	) {
		t.Fatalf(
			"ignored action files = %#v, want %#v",
			result.IgnoredActionFiles,
			wantIgnored,
		)
	}
}

func TestFindSeparateBranchesCanTrigger(
	t *testing.T,
) {
	root := t.TempDir()

	aDir := filepath.Join(
		root,
		"A",
		"001",
	)

	bDir := filepath.Join(
		root,
		"B",
		"002",
	)

	if err := os.MkdirAll(
		aDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(
		bDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	aFile := filepath.Join(
		aDir,
		".yamr.yaml",
	)

	bFile := filepath.Join(
		bDir,
		".yamr.yaml",
	)

	writeTestFile(
		t,
		aFile,
		`
trigger-yamr-runner: true

yamr-runner:
  sources-label: a
`,
	)

	writeTestFile(
		t,
		bFile,
		`
trigger-yamr-runner: true

yamr-runner:
  sources-label: b
`,
	)

	result, err := Find(
		Options{
			RunDir:     root,
			RepoRoot:   root,
			ActionFile: ".yamr.yaml",
		},
	)
	if err != nil {
		t.Fatalf(
			"Find() error = %v",
			err,
		)
	}

	wantCandidates := []string{
		canonicalTestPath(
			t,
			aFile,
		),
		canonicalTestPath(
			t,
			bFile,
		),
	}

	if len(result.Candidates) !=
		len(wantCandidates) {
		t.Fatalf(
			"candidates = %d, want %d",
			len(result.Candidates),
			len(wantCandidates),
		)
	}

	for i, want := range wantCandidates {
		if result.Candidates[i].ActionFile != want {
			t.Fatalf(
				"candidate[%d] = %q, want %q",
				i,
				result.Candidates[i].ActionFile,
				want,
			)
		}
	}

	if len(result.IgnoredActionFiles) != 0 {
		t.Fatalf(
			"ignored action files = %#v, want none",
			result.IgnoredActionFiles,
		)
	}
}

func TestFindRejectsInvalidTriggerValues(
	t *testing.T,
) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "null",
			value: "null",
		},
		{
			name:  "quoted true",
			value: `"true"`,
		},
		{
			name:  "integer",
			value: "1",
		},
		{
			name:  "sequence",
			value: "[]",
		},
		{
			name:  "mapping",
			value: "{}",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				root := t.TempDir()

				writeTestFile(
					t,
					filepath.Join(
						root,
						".yamr.yaml",
					),
					"trigger-yamr-runner: "+
						test.value+
						"\n"+
						"yamr-runner: {}\n",
				)

				_, err := Find(
					Options{
						RunDir:     root,
						RepoRoot:   root,
						ActionFile: ".yamr.yaml",
					},
				)

				if err == nil {
					t.Fatal(
						"Find() error = nil, want error",
					)
				}

				if !strings.Contains(
					err.Error(),
					"trigger-yamr-runner must be a boolean",
				) {
					t.Fatalf(
						"Find() error = %q, want boolean trigger error",
						err,
					)
				}
			},
		)
	}
}

func TestFindIgnoresTriggerInDefaults(
	t *testing.T,
) {
	root := t.TempDir()

	defaultsFile := filepath.Join(
		root,
		"defaults.yaml",
	)

	writeTestFile(
		t,
		defaultsFile,
		`
trigger-yamr-runner: true

yamr-runner:
  env:
    FROM_DEFAULTS: value
`,
	)

	defaults, err := config.Load(
		defaultsFile,
	)
	if err != nil {
		t.Fatalf(
			"config.Load() error = %v",
			err,
		)
	}

	result, err := Find(
		Options{
			RunDir:     root,
			RepoRoot:   root,
			ActionFile: ".yamr.yaml",
			Defaults:   defaults,
		},
	)
	if err != nil {
		t.Fatalf(
			"Find() error = %v",
			err,
		)
	}

	if len(result.Candidates) != 0 {
		t.Fatalf(
			"candidates = %d, want 0",
			len(result.Candidates),
		)
	}

	if len(result.IgnoredActionFiles) != 0 {
		t.Fatalf(
			"ignored action files = %d, want 0",
			len(result.IgnoredActionFiles),
		)
	}
}

func TestFindResolvesImmediateTokensThroughInheritance(
	t *testing.T,
) {
	root := t.TempDir()

	sourcesDir := filepath.Join(
		root,
		"sources",
	)

	actionDir := filepath.Join(
		root,
		"some",
		"action",
	)

	if err := os.MkdirAll(
		sourcesDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(
		actionDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	writeTestFile(
		t,
		filepath.Join(
			root,
			".yamr.yaml",
		),
		`
yamr-runner:
  env:
    SOURCES_DIR: /$yamr_sources_dir$
`,
	)

	writeTestFile(
		t,
		filepath.Join(
			actionDir,
			".yamr.yaml",
		),
		`
trigger-yamr-runner: true

yamr-runner:
  sources-label: test
  env:
    THIS_DIR: /$this_dir$
    RELATIVE_DIR: /$this_dir_rel_to_repo_root$/
`,
	)

	result, err := Find(
		Options{
			RunDir:         root,
			RepoRoot:       root,
			YamrSourcesDir: sourcesDir,
			ActionFile:     ".yamr.yaml",
		},
	)
	if err != nil {
		t.Fatalf(
			"Find() error = %v",
			err,
		)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf(
			"candidates = %d, want 1",
			len(result.Candidates),
		)
	}

	var decoded struct {
		YamrRunner struct {
			Env map[string]string `yaml:"env"`
		} `yaml:"yamr-runner"`
	}

	if err := result.Candidates[0].Config.Decode(
		&decoded,
	); err != nil {
		t.Fatalf(
			"Decode() error = %v",
			err,
		)
	}

	wantSourcesDir := canonicalTestPath(
		t,
		sourcesDir,
	)

	wantActionDir := canonicalTestPath(
		t,
		actionDir,
	)

	wantEnv := map[string]string{
		"SOURCES_DIR":  wantSourcesDir,
		"THIS_DIR":     wantActionDir,
		"RELATIVE_DIR": "/some/action/",
	}

	if !reflect.DeepEqual(
		decoded.YamrRunner.Env,
		wantEnv,
	) {
		t.Fatalf(
			"env = %#v, want %#v",
			decoded.YamrRunner.Env,
			wantEnv,
		)
	}
}

func TestFindResolvesImmediateTokensFromDefaults(
	t *testing.T,
) {
	root := t.TempDir()

	sourcesDir := filepath.Join(
		root,
		"sources",
	)

	actionDir := filepath.Join(
		root,
		"action",
	)

	if err := os.MkdirAll(
		sourcesDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(
		actionDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	defaultsFile := filepath.Join(
		root,
		"defaults.yaml",
	)

	writeTestFile(
		t,
		defaultsFile,
		`
yamr-runner:
  env:
    SOURCES_DIR: /$yamr_sources_dir$
`,
	)

	defaults, err := config.Load(
		defaultsFile,
	)
	if err != nil {
		t.Fatalf(
			"config.Load() error = %v",
			err,
		)
	}

	writeTestFile(
		t,
		filepath.Join(
			actionDir,
			".yamr.yaml",
		),
		`
trigger-yamr-runner: true

yamr-runner:
  sources:
    - test.yaml
`,
	)

	result, err := Find(
		Options{
			RunDir:         root,
			RepoRoot:       root,
			YamrSourcesDir: sourcesDir,
			ActionFile:     ".yamr.yaml",
			Defaults:       defaults,
		},
	)
	if err != nil {
		t.Fatalf(
			"Find() error = %v",
			err,
		)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf(
			"candidates = %d, want 1",
			len(result.Candidates),
		)
	}

	var decoded struct {
		YamrRunner struct {
			Env map[string]string `yaml:"env"`
		} `yaml:"yamr-runner"`
	}

	if err := result.Candidates[0].Config.Decode(
		&decoded,
	); err != nil {
		t.Fatalf(
			"Decode() error = %v",
			err,
		)
	}

	want := canonicalTestPath(
		t,
		sourcesDir,
	)

	if got := decoded.YamrRunner.Env["SOURCES_DIR"]; got != want {
		t.Fatalf(
			"SOURCES_DIR = %q, want %q",
			got,
			want,
		)
	}
}

func TestFindMergesAncestorRunnerConfig(t *testing.T) {
	root := newTree(t)

	parent := filepath.Join(root, "m4m")
	child := filepath.Join(parent, "polaris")
	leaf := filepath.Join(child, "prod")

	mkdir(t, leaf)

	writeAction(t, parent, `
yamr-runner:
  env:
    ENTITY: m4m
    SHARED: parent
`)

	writeAction(t, child, `
yamr-runner:
  env:
    PLATFORM: polaris
    SHARED: child
`)

	writeAction(t, leaf, `
trigger-yamr-runner: true
yamr-runner:
  env:
    STACK: prod
`)

	candidates := findCandidates(t, root)

	if len(candidates) != 1 {
		t.Fatalf(
			"got %d candidates, want 1",
			len(candidates),
		)
	}

	env := nestedMapping(
		t,
		candidates[0].Config,
		"yamr-runner",
		"env",
	)

	assertMappingScalar(t, env, "ENTITY", "m4m")
	assertMappingScalar(t, env, "PLATFORM", "polaris")
	assertMappingScalar(t, env, "STACK", "prod")
	assertMappingScalar(t, env, "SHARED", "child")
}

func TestFindMergesRunnerDefaults(t *testing.T) {
	root := newTree(t)

	writeAction(t, root, `
trigger-yamr-runner: true
yamr-runner:
  env:
    ENTITY: m4m
`)

	defaults := parseYAML(t, `
yamr-runner:
  env:
    FROM_DEFAULTS: yes
    ENTITY: default
`)

	result, err := Find(Options{
		RunDir:         root,
		RepoRoot:       root,
		YamrSourcesDir: root,
		ActionFile:     ".yamr.yaml",
		Defaults:       defaults,
	})
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	env := nestedMapping(
		t,
		result.Candidates[0].Config,
		"yamr-runner",
		"env",
	)

	assertMappingScalar(t, env, "FROM_DEFAULTS", "yes")
	assertMappingScalar(t, env, "ENTITY", "m4m")
}

func TestFindResolvesImmediateTokensBeforeMerging(t *testing.T) {
	root := newTree(t)

	parent := filepath.Join(root, "parent")
	child := filepath.Join(parent, "child")

	mkdir(t, child)

	writeAction(t, parent, `
yamr-runner:
  env:
    PARENT_DIR: /$this_dir$
`)

	writeAction(t, child, `
trigger-yamr-runner: true
yamr-runner:
  env:
    CHILD_DIR: /$this_dir$
`)

	candidates := findCandidates(t, root)

	if len(candidates) != 1 {
		t.Fatalf(
			"got %d candidates, want 1",
			len(candidates),
		)
	}

	env := nestedMapping(
		t,
		candidates[0].Config,
		"yamr-runner",
		"env",
	)

	assertMappingScalar(
		t,
		env,
		"PARENT_DIR",
		canonicalTestPath(t, parent),
	)

	assertMappingScalar(
		t,
		env,
		"CHILD_DIR",
		canonicalTestPath(t, child),
	)
}

func TestFindLeavesActionDirTokenUnresolved(t *testing.T) {
	root := newTree(t)

	writeAction(t, root, `
trigger-yamr-runner: true
yamr-runner:
  env:
    OUTPUT: /$action_dir$/.generated
`)

	candidates := findCandidates(t, root)

	env := nestedMapping(
		t,
		candidates[0].Config,
		"yamr-runner",
		"env",
	)

	assertMappingScalar(
		t,
		env,
		"OUTPUT",
		"/$action_dir$/.generated",
	)
}

func TestFindFailsForInvalidActionYAML(t *testing.T) {
	root := newTree(t)

	path := filepath.Join(root, ".yamr.yaml")

	writeFile(t, path, `
trigger-yamr-runner: true
yamr-runner: [
`)

	_, err := Find(Options{
		RunDir:         root,
		RepoRoot:       root,
		YamrSourcesDir: root,
		ActionFile:     ".yamr.yaml",
		Defaults:       emptyDefaults(t),
	})

	if err == nil {
		t.Fatal("Find() succeeded, want error")
	}
}

func TestFindMergesOnlyRunnerConfigFromAncestors(
	t *testing.T,
) {
	root := t.TempDir()
	child := filepath.Join(
		root,
		"child",
	)

	if err := os.MkdirAll(
		child,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	writeFile(
		t,
		filepath.Join(
			root,
			".yamr.yaml",
		),
		`
ignored-parent:
  value: parent

yamr-runner:
  env:
    ENTITY: m4m
`,
	)

	writeFile(
		t,
		filepath.Join(
			child,
			".yamr.yaml",
		),
		`
ignored-child:
  value: child

trigger-yamr-runner: true
yamr-runner:
  env:
    STACK: prod
`,
	)

	result, err := Find(
		Options{
			RunDir:         root,
			RepoRoot:       root,
			YamrSourcesDir: "/sources",
			ActionFile:     ".yamr.yaml",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	candidates := result.Candidates
	if len(candidates) != 1 {
		t.Fatalf(
			"len(candidates) = %d, want 1",
			len(candidates),
		)
	}

	var value map[string]any

	if err := candidates[0].Config.Decode(
		&value,
	); err != nil {
		t.Fatal(err)
	}

	if len(value) != 1 {
		t.Fatalf(
			"top-level config = %#v, want only yamr-runner",
			value,
		)
	}

	runner, ok := value["yamr-runner"].(map[string]any)
	if !ok {
		t.Fatalf(
			"yamr-runner = %#v, want mapping",
			value["yamr-runner"],
		)
	}
	env, ok := runner["env"].(map[string]any)
	if !ok {
		t.Fatalf(
			"yamr-runner.env = %#v, want mapping",
			runner["env"],
		)
	}

	if env["ENTITY"] != "m4m" {
		t.Fatalf(
			"ENTITY = %#v, want m4m",
			env["ENTITY"],
		)
	}

	if env["STACK"] != "prod" {
		t.Fatalf(
			"STACK = %#v, want prod",
			env["STACK"],
		)
	}
}

func TestFindUsesOnlyRunnerConfigFromDefaults(
	t *testing.T,
) {
	root := t.TempDir()

	writeFile(
		t,
		filepath.Join(
			root,
			".yamr.yaml",
		),
		`
trigger-yamr-runner: true
yamr-runner:
  env:
    ENTITY: m4m
`,
	)

	defaults := parseYAML(
		t,
		`
yamr-sources-dir: /wrong
yamr-source-labels: /wrong/labels.yaml

yamr-runner:
  env:
    PLATFORM: polaris
`,
	)

	result, err := Find(
		Options{
			RunDir:         root,
			RepoRoot:       root,
			YamrSourcesDir: "/real/sources",
			ActionFile:     ".yamr.yaml",
			Defaults:       defaults,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	candidates := result.Candidates

	if len(candidates) != 1 {
		t.Fatalf(
			"len(candidates) = %d, want 1",
			len(candidates),
		)
	}

	var value map[string]any

	if err := candidates[0].Config.Decode(
		&value,
	); err != nil {
		t.Fatal(err)
	}

	if len(value) != 1 {
		t.Fatalf(
			"top-level config = %#v, want only yamr-runner",
			value,
		)
	}

	// for good measure, make sure yamr-runner contents are as expected
	runner, ok := value["yamr-runner"].(map[string]any)
	if !ok {
		t.Fatalf(
			"yamr-runner = %#v, want mapping",
			value["yamr-runner"],
		)
	}
	env, ok := runner["env"].(map[string]any)
	if !ok {
		t.Fatalf(
			"yamr-runner.env = %#v, want mapping",
			runner["env"],
		)
	}

	if env["ENTITY"] != "m4m" {
		t.Fatalf(
			"ENTITY = %#v, want m4m",
			env["ENTITY"],
		)
	}
	if env["PLATFORM"] != "polaris" {
		t.Fatalf(
			"PLATFORM = %#v, want polaris",
			env["PLATFORM"],
		)
	}
}

func TestFindIgnoresTopLevelKeysOutsideRunnerConfig(t *testing.T) {
	root := t.TempDir()

	writeFile(
		t,
		filepath.Join(
			root,
			".yamr.yaml",
		),
		`
trigger-yamr-runner: true

ignored:
  value: $definitely_unknown_token$

yamr-runner:
  sources:
    - test.yaml
`,
	)
	candidates := findCandidates(t, root)

	if len(candidates) != 1 {
		t.Fatalf(
			"len(candidates) = %d, want 1",
			len(candidates),
		)
	}

	var value map[string]any

	if err := candidates[0].Config.Decode(
		&value,
	); err != nil {
		t.Fatal(err)
	}

	if len(value) != 1 {
		t.Fatalf(
			"top-level config = %#v, want only yamr-runner",
			value,
		)
	}

	runner, ok := value["yamr-runner"].(map[string]any)
	if !ok {
		t.Fatalf(
			"yamr-runner = %#v, want mapping",
			value["yamr-runner"],
		)
	}

	if len(runner) != 1 {
		t.Fatalf(
			"yamr-runner = %#v but should only contain one element - sources ",
			runner,
		)
	}

	sources, ok := runner["sources"].([]any)
	if !ok {
		t.Fatalf(
			"yamr-runner.sources = %#v, want list",
			runner["sources"],
		)
	}
	if len(sources) != 1 {
		t.Fatalf(
			"yamr-runner.sources = %#v but should only contain one element - test.yaml ",
			sources,
		)
	}

	source_file, ok := sources[0].(string)
	if !ok {
		t.Fatalf(
			"yamr-runner.sources[0] = %#v, want string",
			sources[0],
		)
	}
	if source_file != "test.yaml" {
		t.Fatalf(
			"yamr-runner.sources[0] = %q, want 'test.yaml'",
			sources[0],
		)
	}
}

func writeTestFile(
	t *testing.T,
	path string,
	content string,
) {
	t.Helper()

	if err := os.MkdirAll(
		filepath.Dir(path),
		0o755,
	); err != nil {
		t.Fatalf(
			"MkdirAll(%q) error = %v",
			filepath.Dir(path),
			err,
		)
	}

	if err := os.WriteFile(
		path,
		[]byte(content),
		0o644,
	); err != nil {
		t.Fatalf(
			"WriteFile(%q) error = %v",
			path,
			err,
		)
	}
}

func canonicalTestPath(
	t *testing.T,
	path string,
) string {
	t.Helper()

	result, err := canonicalPath(
		path,
	)
	if err != nil {
		t.Fatalf(
			"canonicalPath(%q) error = %v",
			path,
			err,
		)
	}

	return result
}

func newTree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	return canonicalTestPath(t, root)
}

func findCandidates(t *testing.T, root string) []Candidate {
	t.Helper()

	result, err := Find(Options{
		RunDir:         root,
		RepoRoot:       root,
		YamrSourcesDir: root,
		ActionFile:     ".yamr.yaml",
		Defaults:       emptyDefaults(t),
	})
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	return result.Candidates
}

func emptyDefaults(t *testing.T) *yaml.Node {
	t.Helper()
	return parseYAML(t, "{}")
}

func writeAction(
	t *testing.T,
	dir string,
	contents string,
) {
	t.Helper()

	mkdir(t, dir)

	writeFile(
		t,
		filepath.Join(dir, ".yamr.yaml"),
		contents,
	)
}

func mkdir(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf(
			"os.MkdirAll(%q): %v",
			path,
			err,
		)
	}
}

func writeFile(
	t *testing.T,
	path string,
	contents string,
) {
	t.Helper()

	if err := os.WriteFile(
		path,
		[]byte(contents),
		0o600,
	); err != nil {
		t.Fatalf(
			"os.WriteFile(%q): %v",
			path,
			err,
		)
	}
}

func parseYAML(
	t *testing.T,
	value string,
) *yaml.Node {
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

func assertCandidateDirs(
	t *testing.T,
	candidates []Candidate,
	want ...string,
) {
	t.Helper()

	got := make([]string, len(candidates))
	for i, candidate := range candidates {
		got[i] = candidate.ActionDir
	}

	for i := range want {
		want[i] = canonicalTestPath(t, want[i])
	}

	sort.Strings(got)
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf(
			"candidate directories:\n got: %q\nwant: %q",
			got,
			want,
		)
	}
}

func nestedMapping(
	t *testing.T,
	document *yaml.Node,
	keys ...string,
) *yaml.Node {
	t.Helper()

	if document.Kind != yaml.DocumentNode ||
		len(document.Content) != 1 {
		t.Fatal("invalid YAML document")
	}

	current := document.Content[0]

	for _, key := range keys {
		if current.Kind != yaml.MappingNode {
			t.Fatalf(
				"%q parent is not a mapping",
				key,
			)
		}

		var next *yaml.Node

		for i := 0; i < len(current.Content); i += 2 {
			if current.Content[i].Value == key {
				next = current.Content[i+1]
				break
			}
		}

		if next == nil {
			t.Fatalf("mapping key %q not found", key)
		}

		current = next
	}

	if current.Kind != yaml.MappingNode {
		t.Fatalf(
			"%q is not a mapping",
			keys[len(keys)-1],
		)
	}

	return current
}

func assertMappingScalar(
	t *testing.T,
	mapping *yaml.Node,
	key string,
	want string,
) {
	t.Helper()

	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != key {
			continue
		}

		value := mapping.Content[i+1]

		if value.Kind != yaml.ScalarNode {
			t.Fatalf(
				"%s kind = %v, want scalar",
				key,
				value.Kind,
			)
		}

		if value.Value != want {
			t.Fatalf(
				"%s = %q, want %q",
				key,
				value.Value,
				want,
			)
		}

		return
	}

	t.Fatalf("mapping key %q not found", key)
}
