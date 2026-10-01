package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"jinal--shah/yamr-run/internal/cli"
	"jinal--shah/yamr-run/internal/runner"
)

func TestBuildCreatesExecutionPlan(
	t *testing.T,
) {
	root := t.TempDir()

	initGitRepository(
		t,
		root,
	)

	configFile := filepath.Join(
		root,
		"runner.yaml",
	)

	labelsFile := filepath.Join(
		root,
		"labels.yaml",
	)

	sourcesDir := filepath.Join(
		root,
		"sources",
	)

	if err := os.MkdirAll(
		sourcesDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	writeFile(
		t,
		configFile,
		`
yamr-sources-dir: /ignored/by/cli
yamr-source-labels: /ignored/by/cli

yamr-runner:
  action:
    run:
      docker:
        image: propero/yamr:test
        entrypoint:
          - yamr
        cmd_sources: $yamr_sources_from_label$
`,
	)

	writeFile(
		t,
		labelsFile,
		`
test:
  - $env.ENTITY$.yaml
`,
	)

	actionDir := filepath.Join(
		root,
		"stack",
	)

	writeFile(
		t,
		filepath.Join(
			actionDir,
			DefaultActionFile,
		),
		`
trigger-yamr-runner: true
yamr-runner:
  sources-label: test
  env:
    ENTITY: m4m
`,
	)

	result, err := Build(
		context.Background(),
		BuildOptions{
			CLI: cli.Options{
				ConfigFile:       configFile,
				YamrSourcesDir:   sourcesDir,
				YamrSourceLabels: labelsFile,
			},
			RunDir:  root,
			HostEnv: nil,
		},
	)
	if err != nil {
		t.Fatalf(
			"Build() error = %v",
			err,
		)
	}

	if result.Repository.Root != canonicalTestPath(t, root) {
		t.Fatalf(
			"repository root = %q, want %q",
			result.Repository.Root,
			root,
		)
	}

	if len(result.Plan.Actions) != 1 {
		t.Fatalf(
			"plan actions = %d, want 1",
			len(result.Plan.Actions),
		)
	}

	planned := result.Plan.Actions[0]

	if planned.Docker.Image != "propero/yamr:test" {
		t.Fatalf(
			"docker image = %q",
			planned.Docker.Image,
		)
	}

	if len(planned.Docker.CmdSources) != 1 ||
		planned.Docker.CmdSources[0] != "m4m.yaml" {
		t.Fatalf(
			"docker sources = %#v, want [m4m.yaml]",
			planned.Docker.CmdSources,
		)
	}
}

func TestBuildExcludesSkippedActions(
	t *testing.T,
) {
	root := t.TempDir()

	initGitRepository(
		t,
		root,
	)

	configFile := filepath.Join(
		root,
		"runner.yaml",
	)

	labelsFile := filepath.Join(
		root,
		"labels.yaml",
	)

	writeFile(
		t,
		configFile,
		`
yamr-sources-dir: /sources
yamr-source-labels: /labels.yaml

yamr-runner:
  action:
    run:
      docker:
        image: propero/yamr:test
        entrypoint:
          - yamr
        cmd_sources: $yamr_sources_from_label$
`,
	)

	writeFile(
		t,
		labelsFile,
		`
unused:
  - unused.yaml
`,
	)

	writeFile(
		t,
		filepath.Join(
			root,
			"skip",
			DefaultActionFile,
		),
		`
yamr-runner:
`,
	)

	result, err := Build(
		context.Background(),
		BuildOptions{
			CLI: cli.Options{
				ConfigFile:       configFile,
				YamrSourcesDir:   "/sources",
				YamrSourceLabels: labelsFile,
			},
			RunDir: root,
		},
	)
	if err != nil {
		t.Fatalf(
			"Build() error = %v",
			err,
		)
	}

	if len(result.Plan.Actions) != 0 {
		t.Fatalf(
			"plan actions = %d, want 0",
			len(result.Plan.Actions),
		)
	}
}

// I want this test to confirm no-one shoved the no-prompts gating
// into Build() as it doesn't belong there.
func TestBuildDirtyRepositoryStillSucceeds(
	t *testing.T,
) {
	root := t.TempDir()

	initGitRepository(
		t,
		root,
	)

	configFile := filepath.Join(
		root,
		"runner.yaml",
	)

	labelsFile := filepath.Join(
		root,
		"labels.yaml",
	)

	writeFile(
		t,
		configFile,
		`
yamr-sources-dir: /sources
yamr-source-labels: /labels.yaml
`,
	)

	writeFile(
		t,
		labelsFile,
		`{}
`,
	)

	// Make the repository dirty after its initial commit.
	writeFile(
		t,
		filepath.Join(
			root,
			"README.md",
		),
		"modified\n",
	)

	result, err := Build(
		context.Background(),
		BuildOptions{
			CLI: cli.Options{
				ConfigFile:       configFile,
				YamrSourcesDir:   "/sources",
				YamrSourceLabels: labelsFile,
			},
			RunDir: root,
		},
	)
	if err != nil {
		t.Fatalf(
			"Build() error = %v",
			err,
		)
	}

	if !result.GitStatus.Dirty {
		t.Fatal(
			"GitStatus.Dirty = false, want true",
		)
	}
}

func TestBuildRejectsNilContext(
	t *testing.T,
) {
	_, err := Build(
		nil,
		BuildOptions{},
	)

	if err == nil {
		t.Fatal(
			"Build() succeeded, want error",
		)
	}

	if !strings.Contains(
		err.Error(),
		"context must not be nil",
	) {
		t.Fatalf(
			"error = %q",
			err,
		)
	}
}

func TestBuildRejectsEmptyRunDir(
	t *testing.T,
) {
	_, err := Build(
		context.Background(),
		BuildOptions{},
	)

	if err == nil {
		t.Fatal(
			"Build() succeeded, want error",
		)
	}

	if !strings.Contains(
		err.Error(),
		"run directory must not be empty",
	) {
		t.Fatalf(
			"error = %q",
			err,
		)
	}
}

func TestBuildAncestorInheritanceThroughStack(
	t *testing.T,
) {
	root := t.TempDir()

	initGitRepository(
		t,
		root,
	)

	configFile := filepath.Join(
		root,
		"runner.yaml",
	)

	labelsFile := filepath.Join(
		root,
		"labels.yaml",
	)

	yamrSourcesDir := filepath.Join(
		root,
		"yamr-sources",
	)

	if err := os.MkdirAll(
		yamrSourcesDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	//
	// Runner defaults.
	//
	// These should survive all the way through discovery,
	// action compilation, and BuildPlan().
	//
	writeFile(
		t,
		configFile,
		`
yamr-sources-dir: /ignored/config/sources
yamr-source-labels: /ignored/config/labels.yaml

yamr-runner:
  action:
    run:
      docker:
        image: propero/yamr:candidate
        user_group: $uid_me$:$gid_me$
        entrypoint:
          - yamr
        cmd_opts:
          - -c
          - /conf/yamr.yaml
          - -o
          - /dev/null
          - --
        cmd_sources: $yamr_sources_from_label$
        work_dir: /$yamr_sources_dir$
        mounts:
          - /$repo_root$:/$repo_root$
          - /$yamr_sources_dir$:/$yamr_sources_dir$
`,
	)

	//
	// Source label contains deferred $env.*$ tokens.
	//
	writeFile(
		t,
		labelsFile,
		`
vpc-platform:
  - aws_account/$env.ENTITY$.yaml
  - platform/$env.PLATFORM$/common.yaml
  - platform/$env.PLATFORM$/$env.ENTITY$/common.yaml
  - platform/$env.PLATFORM$/$env.ENTITY$/$env.STACK$.yaml
  - platform/$env.PLATFORM$/resource_names.yaml
  - platform/$env.PLATFORM$/stack_tags.yaml
  - platform/$env.PLATFORM$/vpcs/platform.yaml
  - deploy/tf_defaults.yaml
  - deploy/terragrunt_manual_workflow.yaml
  - deploy/vpc/platform/resource_naming.yaml
  - deploy/vpc/platform/deployment_inputs.yaml
`,
	)

	//
	// Build:
	//
	// m4m
	// └── polaris
	//     └── prod
	//         └── vpc
	//             └── platform
	//
	m4mDir := filepath.Join(
		root,
		"m4m",
	)

	polarisDir := filepath.Join(
		m4mDir,
		"polaris",
	)

	prodDir := filepath.Join(
		polarisDir,
		"prod",
	)

	platformDir := filepath.Join(
		prodDir,
		"vpc",
		"platform",
	)

	//
	// ENTITY comes from the highest action ancestor.
	//
	writeFile(
		t,
		filepath.Join(
			m4mDir,
			DefaultActionFile,
		),
		`
yamr-runner:
  env:
    ENTITY: m4m
`,
	)

	//
	// PLATFORM comes from the next ancestor.
	//
	writeFile(
		t,
		filepath.Join(
			polarisDir,
			DefaultActionFile,
		),
		`
yamr-runner:
  env:
    PLATFORM: polaris
`,
	)

	//
	// STACK comes from the next ancestor.
	//
	writeFile(
		t,
		filepath.Join(
			prodDir,
			DefaultActionFile,
		),
		`
yamr-runner:
  env:
    STACK: prod
`,
	)

	//
	// The leaf selects the source label and exercises immediate
	// path tokens.
	//
	writeFile(
		t,
		filepath.Join(
			platformDir,
			DefaultActionFile,
		),
		`
trigger-yamr-runner: true
yamr-runner:
  sources-label: vpc-platform
  env:
    WORK_DIR: /$this_dir$/.generated
    RUN_SRC: /$this_dir_rel_to_repo_root$/
`,
	)

	result, err := Build(
		context.Background(),
		BuildOptions{
			CLI: cli.Options{
				ConfigFile:       configFile,
				YamrSourcesDir:   yamrSourcesDir,
				YamrSourceLabels: labelsFile,
			},
			RunDir: root,
		},
	)
	if err != nil {
		t.Fatalf(
			"Build() error = %v",
			err,
		)
	}

	if len(result.Plan.Actions) != 1 {
		t.Fatalf(
			"plan actions = %d, want 1",
			len(result.Plan.Actions),
		)
	}

	planned := result.Plan.Actions[0]

	if planned.Action == nil {
		t.Fatal(
			"planned action is nil",
		)
	}

	//
	// Discovery should select the deepest action in this branch.
	//
	wantActionDir := canonicalTestPath(
		t,
		platformDir,
	)

	if planned.Action.ActionDir != wantActionDir {
		t.Fatalf(
			"ActionDir = %q, want %q",
			planned.Action.ActionDir,
			wantActionDir,
		)
	}

	wantActionFile := filepath.Join(
		wantActionDir,
		DefaultActionFile,
	)

	if planned.Action.ActionFile != wantActionFile {
		t.Fatalf(
			"ActionFile = %q, want %q",
			planned.Action.ActionFile,
			wantActionFile,
		)
	}

	//
	// Environment should contain values inherited from all three
	// ancestors plus immediate tokens resolved relative to the
	// leaf action file.
	//
	wantEnv := map[string]string{
		"ENTITY":   "m4m",
		"PLATFORM": "polaris",
		"STACK":    "prod",
		"WORK_DIR": filepath.Join(
			wantActionDir,
			".generated",
		),
		"RUN_SRC": "/m4m/polaris/prod/vpc/platform/",
	}

	if !reflect.DeepEqual(
		planned.Action.Env,
		wantEnv,
	) {
		t.Fatalf(
			"action env = %#v\nwant %#v",
			planned.Action.Env,
			wantEnv,
		)
	}

	//
	// Source label entries should have been resolved using the
	// final inherited environment.
	//
	wantSources := []string{
		"aws_account/m4m.yaml",
		"platform/polaris/common.yaml",
		"platform/polaris/m4m/common.yaml",
		"platform/polaris/m4m/prod.yaml",
		"platform/polaris/resource_names.yaml",
		"platform/polaris/stack_tags.yaml",
		"platform/polaris/vpcs/platform.yaml",
		"deploy/tf_defaults.yaml",
		"deploy/terragrunt_manual_workflow.yaml",
		"deploy/vpc/platform/resource_naming.yaml",
		"deploy/vpc/platform/deployment_inputs.yaml",
	}

	if !reflect.DeepEqual(
		planned.Action.Sources,
		wantSources,
	) {
		t.Fatalf(
			"action sources = %#v\nwant %#v",
			planned.Action.Sources,
			wantSources,
		)
	}

	//
	// Now verify those values made it through BuildPlan into the
	// actual Docker command.
	//
	if planned.Docker.Image !=
		"propero/yamr:candidate" {
		t.Fatalf(
			"Docker.Image = %q",
			planned.Docker.Image,
		)
	}

	wantUserGroup := fmt.Sprintf(
		"%d:%d",
		os.Geteuid(),
		os.Getegid(),
	)

	if planned.Docker.UserGroup != wantUserGroup {
		t.Fatalf(
			"Docker.UserGroup = %q, want %q",
			planned.Docker.UserGroup,
			wantUserGroup,
		)
	}

	if planned.Docker.Entrypoint != "yamr" {
		t.Fatalf(
			"Docker.Entrypoint = %q, want yamr",
			planned.Docker.Entrypoint,
		)
	}

	wantCmdOpts := []string{
		"-c",
		"/conf/yamr.yaml",
		"-o",
		"/dev/null",
		"--",
	}

	if !reflect.DeepEqual(
		planned.Docker.CmdOpts,
		wantCmdOpts,
	) {
		t.Fatalf(
			"Docker.CmdOpts = %#v\nwant %#v",
			planned.Docker.CmdOpts,
			wantCmdOpts,
		)
	}

	if !reflect.DeepEqual(
		planned.Docker.CmdSources,
		wantSources,
	) {
		t.Fatalf(
			"Docker.CmdSources = %#v\nwant %#v",
			planned.Docker.CmdSources,
			wantSources,
		)
	}

	wantSourcesDir := canonicalTestPath(
		t,
		yamrSourcesDir,
	)

	if planned.Docker.WorkDir != wantSourcesDir {
		t.Fatalf(
			"Docker.WorkDir = %q, want %q",
			planned.Docker.WorkDir,
			wantSourcesDir,
		)
	}

	//
	// Docker gets only the YAML-configured environment.
	//
	if !reflect.DeepEqual(
		planned.Docker.Env,
		wantEnv,
	) {
		t.Fatalf(
			"Docker.Env = %#v\nwant %#v",
			planned.Docker.Env,
			wantEnv,
		)
	}

	//
	// The defaults also contain immediate repo/source-directory
	// tokens in mounts.
	//
	wantRepoRoot := canonicalTestPath(
		t,
		root,
	)

	wantMounts := []string{
		wantRepoRoot + ":" + wantRepoRoot,
		wantSourcesDir + ":" + wantSourcesDir,
	}

	if !reflect.DeepEqual(
		planned.Docker.Mounts,
		wantMounts,
	) {
		t.Fatalf(
			"Docker.Mounts = %#v\nwant %#v",
			planned.Docker.Mounts,
			wantMounts,
		)
	}
}

func TestBuildKitchenSink(
	t *testing.T,
) {
	root := t.TempDir()

	initGitRepository(
		t,
		root,
	)

	configFile := filepath.Join(
		root,
		"runner.yaml",
	)

	labelsFile := filepath.Join(
		root,
		"labels.yaml",
	)

	yamrSourcesDir := filepath.Join(
		root,
		"yamr-sources",
	)

	if err := os.MkdirAll(
		yamrSourcesDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	//
	// Global defaults.
	//
	writeFile(
		t,
		configFile,
		`
yamr-sources-dir: /ignored/by/cli
yamr-source-labels: /ignored/by/cli

yamr-runner:
  env:
    FROM_DEFAULTS: "yes"
    HOST_OVERRIDE: from-yaml
    HOST_SUPPRESSED: null

  action:
    pre_run:
      - mkdir:
          path: /$action_dir$/.generated
          chmod: "0755"

    run:
      docker:
        image: propero/yamr:candidate
        user_group: $uid_me$:$gid_me$
        entrypoint:
          - yamr
        cmd_opts:
          - -c
          - /conf/yamr.yaml
          - -o
          - /dev/null
          - --
        cmd_sources: $yamr_sources_from_label$
        work_dir: /$yamr_sources_dir$
        mounts:
          - /$repo_root$:/$repo_root$
          - /$yamr_sources_dir$:/$yamr_sources_dir$
      stdout: null
      stderr: null
`,
	)

	writeFile(
		t,
		labelsFile,
		`
vpc-platform:
  - aws_account/$env.ENTITY$.yaml
  - platform/$env.PLATFORM$/$env.STACK$/common.yaml
  - platform/$env.PLATFORM$/vpcs/platform.yaml

staging-app:
  - apps/$env.ENTITY$/$env.PLATFORM$/$env.APP$.yaml
  - environments/$env.STACK$.yaml
`,
	)

	//
	// Root ancestor.
	//
	writeFile(
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

	polarisDir := filepath.Join(
		root,
		"polaris",
	)

	writeFile(
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

	//
	// Production ancestor.
	//
	prodDir := filepath.Join(
		polarisDir,
		"prod",
	)

	writeFile(
		t,
		filepath.Join(
			prodDir,
			".yamr.yaml",
		),
		`
yamr-runner:
  env:
    STACK: prod
`,
	)

	//
	// Labelled production platform action.
	//
	platformDir := filepath.Join(
		prodDir,
		"vpc",
		"platform",
	)

	writeFile(
		t,
		filepath.Join(
			platformDir,
			".yamr.yaml",
		),
		`
trigger-yamr-runner: true
yamr-runner:
  sources-label: vpc-platform

  env:
    ACTION_KIND: platform
    ACTION_DIR_VALUE: /$action_dir$
    THIS_DIR_VALUE: /$this_dir$
    RELATIVE_VALUE: /$this_dir_rel_to_repo_root$/

  action:
    run:
      docker:
        image: propero/yamr:platform
`,
	)

	//
	// Inline-source production action.
	//
	// Because inline sources are selected, it overrides the
	// inherited cmd_sources token accordingly.
	//
	cicdDir := filepath.Join(
		prodDir,
		"vpc",
		"cicd",
	)

	writeFile(
		t,
		filepath.Join(
			cicdDir,
			".yamr.yaml",
		),
		`
trigger-yamr-runner: true
yamr-runner:
  sources:
    - cicd/$env.ENTITY$.yaml
    - cicd/$env.STACK$.yaml

  env:
    ACTION_KIND: cicd

  action:
    run:
      docker:
        image: propero/yamr:cicd
        cmd_sources: $yamr_sources$
`,
	)

	//
	// This branch must not become an action.
	//
	ignoredDir := filepath.Join(
		prodDir,
		"vpc",
		"ignored",
	)

	writeFile(
		t,
		filepath.Join(
			ignoredDir,
			".yamr.yaml",
		),
		`
yamr-runner:
`,
	)

	//
	// Independent staging branch.
	//
	stagingDir := filepath.Join(
		polarisDir,
		"staging",
	)

	writeFile(
		t,
		filepath.Join(
			stagingDir,
			".yamr.yaml",
		),
		`
yamr-runner:
  env:
    STACK: staging
`,
	)

	appDir := filepath.Join(
		stagingDir,
		"app",
	)

	writeFile(
		t,
		filepath.Join(
			appDir,
			".yamr.yaml",
		),
		`
trigger-yamr-runner: true
yamr-runner:
  sources-label: staging-app

  env:
    APP: frontend
    ACTION_KIND: app

  action:
    run:
      docker:
        image: propero/yamr:staging
`,
	)

	result, err := Build(
		context.Background(),
		BuildOptions{
			CLI: cli.Options{
				ConfigFile:       configFile,
				YamrSourcesDir:   yamrSourcesDir,
				YamrSourceLabels: labelsFile,
			},
			RunDir: root,
			HostEnv: []string{
				"HOST_ONLY=host-only",
				"HOST_OVERRIDE=from-host",
				"HOST_SUPPRESSED=must-disappear",
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"Build() error = %v",
			err,
		)
	}

	if len(result.Plan.Actions) != 3 {
		t.Fatalf(
			"plan actions = %d, want 3",
			len(result.Plan.Actions),
		)
	}

	//
	// BuildPlan/discovery order is deterministic by action file.
	// Still, looking them up by ActionDir makes this test less
	// coupled to lexical ordering.
	//
	byDir := make(
		map[string]runner.PlannedAction,
		len(result.Plan.Actions),
	)

	for _, planned := range result.Plan.Actions {
		if planned.Action == nil {
			t.Fatal(
				"plan contains nil action",
			)
		}

		byDir[planned.Action.ActionDir] = planned
	}

	platformDir = canonicalTestPath(
		t,
		platformDir,
	)

	cicdDir = canonicalTestPath(
		t,
		cicdDir,
	)

	appDir = canonicalTestPath(
		t,
		appDir,
	)

	platform, ok := byDir[platformDir]
	if !ok {
		t.Fatalf(
			"no planned action for %q",
			platformDir,
		)
	}

	cicd, ok := byDir[cicdDir]
	if !ok {
		t.Fatalf(
			"no planned action for %q",
			cicdDir,
		)
	}

	stagingApp, ok := byDir[appDir]
	if !ok {
		t.Fatalf(
			"no planned action for %q",
			appDir,
		)
	}

	//
	// LABELLED ACTION
	//

	wantPlatformSources := []string{
		"aws_account/m4m.yaml",
		"platform/polaris/prod/common.yaml",
		"platform/polaris/vpcs/platform.yaml",
	}

	if !reflect.DeepEqual(
		platform.Action.Sources,
		wantPlatformSources,
	) {
		t.Fatalf(
			"platform sources = %#v, want %#v",
			platform.Action.Sources,
			wantPlatformSources,
		)
	}

	if !reflect.DeepEqual(
		platform.Docker.CmdSources,
		wantPlatformSources,
	) {
		t.Fatalf(
			"platform docker sources = %#v, want %#v",
			platform.Docker.CmdSources,
			wantPlatformSources,
		)
	}

	if platform.Docker.Image !=
		"propero/yamr:platform" {
		t.Fatalf(
			"platform image = %q",
			platform.Docker.Image,
		)
	}

	//
	// Inheritance + overrides + immediate/final tokens.
	//
	wantPlatformEnv := map[string]string{
		"FROM_DEFAULTS":    "yes",
		"HOST_OVERRIDE":    "from-yaml",
		"ENTITY":           "m4m",
		"SHARED":           "polaris",
		"PLATFORM":         "polaris",
		"STACK":            "prod",
		"ACTION_KIND":      "platform",
		"ACTION_DIR_VALUE": platformDir,
		"THIS_DIR_VALUE":   platformDir,
		"RELATIVE_VALUE":   "/polaris/prod/vpc/platform/",
	}

	if !reflect.DeepEqual(
		platform.Action.Env,
		wantPlatformEnv,
	) {
		t.Fatalf(
			"platform env = %#v\nwant %#v",
			platform.Action.Env,
			wantPlatformEnv,
		)
	}

	//
	// HOST_ONLY must not leak into Docker.
	// HOST_SUPPRESSED must remain absent because YAML null
	// suppresses host fallback.
	//
	if _, ok := platform.Docker.Env["HOST_ONLY"]; ok {
		t.Fatal(
			"HOST_ONLY leaked into Docker environment",
		)
	}

	if _, ok := platform.Docker.Env["HOST_SUPPRESSED"]; ok {
		t.Fatal(
			"HOST_SUPPRESSED leaked into Docker environment",
		)
	}

	if platform.Docker.Env["HOST_OVERRIDE"] !=
		"from-yaml" {
		t.Fatalf(
			"HOST_OVERRIDE = %q, want from-yaml",
			platform.Docker.Env["HOST_OVERRIDE"],
		)
	}

	//
	// INLINE-SOURCE ACTION
	//
	wantCICDSources := []string{
		"cicd/m4m.yaml",
		"cicd/prod.yaml",
	}

	if !reflect.DeepEqual(
		cicd.Action.Sources,
		wantCICDSources,
	) {
		t.Fatalf(
			"cicd sources = %#v, want %#v",
			cicd.Action.Sources,
			wantCICDSources,
		)
	}

	if !reflect.DeepEqual(
		cicd.Docker.CmdSources,
		wantCICDSources,
	) {
		t.Fatalf(
			"cicd docker sources = %#v, want %#v",
			cicd.Docker.CmdSources,
			wantCICDSources,
		)
	}

	if cicd.Docker.Image != "propero/yamr:cicd" {
		t.Fatalf(
			"cicd image = %q",
			cicd.Docker.Image,
		)
	}

	//
	// INDEPENDENT STAGING BRANCH
	//
	wantStagingSources := []string{
		"apps/m4m/polaris/frontend.yaml",
		"environments/staging.yaml",
	}

	if !reflect.DeepEqual(
		stagingApp.Action.Sources,
		wantStagingSources,
	) {
		t.Fatalf(
			"staging sources = %#v, want %#v",
			stagingApp.Action.Sources,
			wantStagingSources,
		)
	}

	if stagingApp.Action.Env["STACK"] != "staging" {
		t.Fatalf(
			"staging STACK = %q, want staging",
			stagingApp.Action.Env["STACK"],
		)
	}

	if stagingApp.Action.Env["SHARED"] != "polaris" {
		t.Fatalf(
			"staging SHARED = %q, want polaris",
			stagingApp.Action.Env["SHARED"],
		)
	}

	if stagingApp.Docker.Image !=
		"propero/yamr:staging" {
		t.Fatalf(
			"staging image = %q",
			stagingApp.Docker.Image,
		)
	}

	//
	// Defaults should have compiled pre_run for every action.
	//
	for name, planned := range map[string]runner.PlannedAction{
		"platform": platform,
		"cicd":     cicd,
		"staging":  stagingApp,
	} {
		if len(planned.PreRun) != 1 {
			t.Fatalf(
				"%s pre_run operations = %d, want 1",
				name,
				len(planned.PreRun),
			)
		}
	}

	//
	// Invocation-wide immediate defaults.
	//
	wantSourcesDir := canonicalTestPath(
		t,
		yamrSourcesDir,
	)

	wantRepoRoot := canonicalTestPath(
		t,
		root,
	)

	for name, planned := range map[string]runner.PlannedAction{
		"platform": platform,
		"cicd":     cicd,
		"staging":  stagingApp,
	} {
		if planned.Docker.WorkDir != wantSourcesDir {
			t.Fatalf(
				"%s workdir = %q, want %q",
				name,
				planned.Docker.WorkDir,
				wantSourcesDir,
			)
		}

		wantMounts := []string{
			wantRepoRoot + ":" + wantRepoRoot,
			wantSourcesDir + ":" + wantSourcesDir,
		}

		if !reflect.DeepEqual(
			planned.Docker.Mounts,
			wantMounts,
		) {
			t.Fatalf(
				"%s mounts = %#v, want %#v",
				name,
				planned.Docker.Mounts,
				wantMounts,
			)
		}
	}
}

func TestBuildReturnsIgnoredActionFiles(
	t *testing.T,
) {
	ctx := context.Background()
	root := t.TempDir()

	initGitRepository(
		t,
		root,
	)

	sourcesDir := filepath.Join(
		root,
		"sources",
	)
	if err := os.MkdirAll(
		sourcesDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(
		root,
		"runner.yaml",
	)
	labelsFile := filepath.Join(
		root,
		"labels.yaml",
	)

	writeFile(
		t,
		configFile,
		`
yamr-runner:
  action:
    run:
      docker:
        image: test-image
        entrypoint:
        - yamr
        cmd_sources: $yamr_sources_from_label$
`,
	)

	writeFile(
		t,
		labelsFile,
		`
test-sources:
  - test.yaml
`,
	)

	actionDir := filepath.Join(
		root,
		"action",
	)
	descendantDir := filepath.Join(
		actionDir,
		"descendant",
	)

	triggerFile := filepath.Join(
		actionDir,
		".yamr.yaml",
	)
	ignoredFile := filepath.Join(
		descendantDir,
		".yamr.yaml",
	)

	writeFile(
		t,
		triggerFile,
		`
trigger-yamr-runner: true

yamr-runner:
  sources-label: test-sources
`,
	)

	//
	// This file is beneath a triggered action and therefore must
	// be reported as ignored rather than discovered as another
	// action.
	//
	writeFile(
		t,
		ignoredFile,
		`
trigger-yamr-runner: true

yamr-runner:
  sources-label: test-sources
`,
	)

	result, err := Build(
		ctx,
		BuildOptions{
			CLI: cli.Options{
				ConfigFile:       configFile,
				YamrSourcesDir:   sourcesDir,
				YamrSourceLabels: labelsFile,
			},
			RunDir: root,
		},
	)
	if err != nil {
		t.Fatalf(
			"Build() error = %v",
			err,
		)
	}

	if len(result.Plan.Actions) != 1 {
		t.Fatalf(
			"plan actions = %d, want 1",
			len(result.Plan.Actions),
		)
	}

	if len(result.IgnoredActionFiles) != 1 {
		t.Fatalf(
			"ignored action files = %d, want 1: %#v",
			len(result.IgnoredActionFiles),
			result.IgnoredActionFiles,
		)
	}

	ignored := result.IgnoredActionFiles[0]

	wantActionFile := canonicalTestPath(
		t,
		ignoredFile,
	)
	wantTriggeredBy := canonicalTestPath(
		t,
		triggerFile,
	)

	if ignored.ActionFile != wantActionFile {
		t.Errorf(
			"ignored ActionFile = %q, want %q",
			ignored.ActionFile,
			wantActionFile,
		)
	}

	if ignored.TriggeredBy != wantTriggeredBy {
		t.Errorf(
			"ignored TriggeredBy = %q, want %q",
			ignored.TriggeredBy,
			wantTriggeredBy,
		)
	}
}

func writeFile(
	t *testing.T,
	path string,
	content string,
) {
	t.Helper()

	if err := os.MkdirAll(
		filepath.Dir(path),
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		path,
		[]byte(content),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
}

func initGitRepository(
	t *testing.T,
	dir string,
) {
	t.Helper()

	runGit(
		t,
		dir,
		"init",
	)

	runGit(
		t,
		dir,
		"config",
		"user.email",
		"test@jinal--shah",
	)

	runGit(
		t,
		dir,
		"config",
		"user.name",
		"Test User",
	)

	writeFile(
		t,
		filepath.Join(
			dir,
			"README.md",
		),
		"test\n",
	)

	runGit(
		t,
		dir,
		"add",
		".",
	)

	runGit(
		t,
		dir,
		"commit",
		"-m",
		"initial",
	)
}

func runGit(
	t *testing.T,
	dir string,
	args ...string,
) {
	t.Helper()

	commandArgs := append(
		[]string{
			"-C",
			dir,
		},
		args...,
	)

	command := exec.Command(
		"git",
		commandArgs...,
	)

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"git %v failed: %v\n%s",
			args,
			err,
			output,
		)
	}
}

func canonicalTestPath(
	t *testing.T,
	path string,
) string {
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
