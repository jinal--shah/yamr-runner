package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jinal--shah/yamr-run/internal/action"
)

func TestBuildPlan(t *testing.T) {
	root := t.TempDir()

	output := filepath.Join(
		root,
		".generated",
	)

	a := plannedTestAction(
		t,
		filepath.Join(
			root,
			".yamr.yaml",
		),
		`
yamr-runner:
  action:
    pre_run:
      - mv_to_tmp:
          path: `+output+`
      - mkdir:
          path: `+output+`
    run:
      docker:
        image: propero/yamr:candidate
        entrypoint:
          - yamr
        cmd_opts:
          - -c
          - /conf/yamr.yaml
          - --
        cmd_sources:
          - /sources/one.yaml
          - /sources/two.yaml
        work_dir: /sources
        mounts:
          - /repo:/repo
`,
	)

	plan, err := BuildPlan(
		[]*action.Action{a},
	)
	if err != nil {
		t.Fatalf(
			"BuildPlan() error = %v",
			err,
		)
	}

	if len(plan.Actions) != 1 {
		t.Fatalf(
			"len(Plan.Actions) = %d, want 1",
			len(plan.Actions),
		)
	}

	planned := plan.Actions[0]

	if planned.Action != a {
		t.Fatal(
			"planned action does not reference input action",
		)
	}

	if len(planned.PreRun) != 2 {
		t.Fatalf(
			"len(PreRun) = %d, want 2",
			len(planned.PreRun),
		)
	}

	if planned.Docker.Image !=
		"propero/yamr:candidate" {
		t.Fatalf(
			"Docker.Image = %q",
			planned.Docker.Image,
		)
	}

	if planned.Docker.Entrypoint != "yamr" {
		t.Fatalf(
			"Docker.Entrypoint = %q, want yamr",
			planned.Docker.Entrypoint,
		)
	}

	if len(planned.Docker.CmdSources) != 2 {
		t.Fatalf(
			"len(Docker.CmdSources) = %d, want 2",
			len(planned.Docker.CmdSources),
		)
	}
}

func TestBuildPlanEmpty(t *testing.T) {
	plan, err := BuildPlan(nil)
	if err != nil {
		t.Fatalf(
			"BuildPlan() error = %v",
			err,
		)
	}

	if len(plan.Actions) != 0 {
		t.Fatalf(
			"len(Plan.Actions) = %d, want 0",
			len(plan.Actions),
		)
	}
}

func TestBuildPlanRejectsNilAction(t *testing.T) {
	_, err := BuildPlan(
		[]*action.Action{nil},
	)

	if err == nil {
		t.Fatal(
			"BuildPlan() succeeded, want error",
		)
	}

	assertPlanErrorContains(
		t,
		err,
		"action 0 is nil",
	)
}

func TestBuildPlanRejectsInvalidPreRun(
	t *testing.T,
) {
	a := plannedTestAction(
		t,
		"/repo/.yamr.yaml",
		`
yamr-runner:
  action:
    pre_run:
      - mkdir:
          path: relative/path
    run:
      docker:
        image: propero/yamr:candidate
        entrypoint:
          - yamr
        cmd_sources:
          - source.yaml
`,
	)

	_, err := BuildPlan(
		[]*action.Action{a},
	)

	if err == nil {
		t.Fatal(
			"BuildPlan() succeeded, want error",
		)
	}

	assertPlanErrorContains(
		t,
		err,
		"compile pre_run",
	)

	assertPlanErrorContains(
		t,
		err,
		"must be absolute",
	)
}

func TestBuildPlanRejectsInvalidDockerConfig(
	t *testing.T,
) {
	a := plannedTestAction(
		t,
		"/repo/.yamr.yaml",
		`
yamr-runner:
  action:
    run:
      docker:
        entrypoint:
          - yamr
        cmd_sources:
          - source.yaml
`,
	)

	_, err := BuildPlan(
		[]*action.Action{a},
	)

	if err == nil {
		t.Fatal(
			"BuildPlan() succeeded, want error",
		)
	}

	assertPlanErrorContains(
		t,
		err,
		"compile docker configuration",
	)

	assertPlanErrorContains(
		t,
		err,
		"image",
	)
}

func TestValidatePlanRejectsDuplicateMovePathInSameAction(
	t *testing.T,
) {
	root := t.TempDir()

	source := filepath.Join(
		root,
		".generated",
	)

	a := &action.Action{
		ActionFile: filepath.Join(
			root,
			".yamr.yaml",
		),
	}

	plan := Plan{
		Actions: []PlannedAction{
			{
				Action: a,
				PreRun: []PreRunOperation{
					MoveToTmpOperation{
						Path: source,
					},
					MoveToTmpOperation{
						Path: source,
					},
				},
			},
		},
	}

	err := ValidatePlan(plan)
	if err == nil {
		t.Fatal(
			"ValidatePlan() succeeded, want error",
		)
	}

	assertPlanErrorContains(
		t,
		err,
		"duplicate mv_to_tmp destination",
	)

	assertPlanErrorContains(
		t,
		err,
		source,
	)
}

func TestValidatePlanRejectsDuplicateMovePathAcrossActions(
	t *testing.T,
) {
	root := t.TempDir()

	source := filepath.Join(
		root,
		".generated",
	)

	firstFile := filepath.Join(
		root,
		"one",
		".yamr.yaml",
	)

	secondFile := filepath.Join(
		root,
		"two",
		".yamr.yaml",
	)

	plan := Plan{
		Actions: []PlannedAction{
			{
				Action: &action.Action{
					ActionFile: firstFile,
				},
				PreRun: []PreRunOperation{
					MoveToTmpOperation{
						Path: source,
					},
				},
			},
			{
				Action: &action.Action{
					ActionFile: secondFile,
				},
				PreRun: []PreRunOperation{
					MoveToTmpOperation{
						Path: source,
					},
				},
			},
		},
	}

	err := ValidatePlan(plan)
	if err == nil {
		t.Fatal(
			"ValidatePlan() succeeded, want error",
		)
	}

	assertPlanErrorContains(
		t,
		err,
		firstFile,
	)

	assertPlanErrorContains(
		t,
		err,
		secondFile,
	)
}

func TestValidatePlanRejectsLexicallyDifferentDuplicatePaths(
	t *testing.T,
) {
	root := t.TempDir()

	first := filepath.Join(
		root,
		"repo",
		".generated",
	)

	second := filepath.Join(
		root,
		"repo",
		"foo",
		"..",
		".generated",
	)

	plan := Plan{
		Actions: []PlannedAction{
			{
				Action: &action.Action{
					ActionFile: "one.yaml",
				},
				PreRun: []PreRunOperation{
					MoveToTmpOperation{
						Path: first,
					},
				},
			},
			{
				Action: &action.Action{
					ActionFile: "two.yaml",
				},
				PreRun: []PreRunOperation{
					MoveToTmpOperation{
						Path: second,
					},
				},
			},
		},
	}

	err := ValidatePlan(plan)
	if err == nil {
		t.Fatal(
			"ValidatePlan() succeeded, want error",
		)
	}

	assertPlanErrorContains(
		t,
		err,
		"duplicate mv_to_tmp destination",
	)
}

func TestValidatePlanRejectsSymlinkAliases(
	t *testing.T,
) {
	root := t.TempDir()

	realDir := filepath.Join(
		root,
		"real",
	)

	if err := os.MkdirAll(
		realDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	linkDir := filepath.Join(
		root,
		"link",
	)

	if err := os.Symlink(
		realDir,
		linkDir,
	); err != nil {
		t.Skipf(
			"cannot create symlink: %v",
			err,
		)
	}

	first := filepath.Join(
		realDir,
		".generated",
	)

	second := filepath.Join(
		linkDir,
		".generated",
	)

	plan := Plan{
		Actions: []PlannedAction{
			{
				Action: &action.Action{
					ActionFile: "one.yaml",
				},
				PreRun: []PreRunOperation{
					MoveToTmpOperation{
						Path: first,
					},
				},
			},
			{
				Action: &action.Action{
					ActionFile: "two.yaml",
				},
				PreRun: []PreRunOperation{
					MoveToTmpOperation{
						Path: second,
					},
				},
			},
		},
	}

	err := ValidatePlan(plan)
	if err == nil {
		t.Fatal(
			"ValidatePlan() succeeded, want error",
		)
	}

	assertPlanErrorContains(
		t,
		err,
		"duplicate mv_to_tmp destination",
	)
}

func TestCanonicalPathExistingPath(t *testing.T) {
	root := t.TempDir()

	path := filepath.Join(
		root,
		"repo",
	)

	if err := os.MkdirAll(
		path,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	got, err := canonicalPath(path)
	if err != nil {
		t.Fatalf(
			"canonicalPath() error = %v",
			err,
		)
	}

	want, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Fatalf(
			"canonicalPath() = %q, want %q",
			got,
			want,
		)
	}
}

func TestCanonicalPathNonExistingSuffix(
	t *testing.T,
) {
	root := t.TempDir()

	existing := filepath.Join(
		root,
		"repo",
	)

	if err := os.MkdirAll(
		existing,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(
		existing,
		"one",
		"two",
		".generated",
	)

	got, err := canonicalPath(path)
	if err != nil {
		t.Fatalf(
			"canonicalPath() error = %v",
			err,
		)
	}

	want, _ := canonicalPath(filepath.Join(
		existing,
		"one",
		"two",
		".generated",
	))

	if got != want {
		t.Fatalf(
			"canonicalPath() = %q, want %q",
			got,
			want,
		)
	}
}

func TestCanonicalPathResolvesSymlinkBeforeMissingSuffix(
	t *testing.T,
) {
	root := t.TempDir()

	realDir := filepath.Join(
		root,
		"real",
	)

	if err := os.MkdirAll(
		realDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	linkDir := filepath.Join(
		root,
		"link",
	)

	if err := os.Symlink(
		realDir,
		linkDir,
	); err != nil {
		t.Skipf(
			"cannot create symlink: %v",
			err,
		)
	}

	path := filepath.Join(
		linkDir,
		"does",
		"not",
		"exist",
	)

	got, err := canonicalPath(path)
	if err != nil {
		t.Fatalf(
			"canonicalPath() error = %v",
			err,
		)
	}

	want, _ := canonicalPath(filepath.Join(
		realDir,
		"does",
		"not",
		"exist",
	))

	if got != want {
		t.Fatalf(
			"canonicalPath() = %q, want %q",
			got,
			want,
		)
	}
}

func TestValidatePlanAllowsDistinctMovePaths(
	t *testing.T,
) {
	root := t.TempDir()

	plan := Plan{
		Actions: []PlannedAction{
			{
				Action: &action.Action{
					ActionFile: "one.yaml",
				},
				PreRun: []PreRunOperation{
					MoveToTmpOperation{
						Path: filepath.Join(
							root,
							"one",
						),
					},
				},
			},
			{
				Action: &action.Action{
					ActionFile: "two.yaml",
				},
				PreRun: []PreRunOperation{
					MoveToTmpOperation{
						Path: filepath.Join(
							root,
							"two",
						),
					},
				},
			},
		},
	}

	if err := ValidatePlan(plan); err != nil {
		t.Fatalf(
			"ValidatePlan() error = %v",
			err,
		)
	}
}

func TestValidatePlanAllowsIdenticalMkdirDuplicates(
	t *testing.T,
) {
	root := t.TempDir()

	path := filepath.Join(
		root,
		"output",
	)

	plan := Plan{
		Actions: []PlannedAction{
			{
				Action: &action.Action{
					ActionFile: "one.yaml",
				},
				PreRun: []PreRunOperation{
					MkdirOperation{
						Path: path,
						Mode: 0o755,
						UID:  1000,
						GID:  1000,
					},
				},
			},
			{
				Action: &action.Action{
					ActionFile: "two.yaml",
				},
				PreRun: []PreRunOperation{
					MkdirOperation{
						Path: path,
						Mode: 0o755,
						UID:  1000,
						GID:  1000,
					},
				},
			},
		},
	}

	if err := ValidatePlan(plan); err != nil {
		t.Fatalf(
			"ValidatePlan() error = %v",
			err,
		)
	}
}

func TestValidatePlanRejectsConflictingMkdirMode(
	t *testing.T,
) {
	root := t.TempDir()

	path := filepath.Join(
		root,
		"output",
	)

	plan := mkdirConflictPlan(
		path,
		MkdirOperation{
			Path: path,
			Mode: 0o755,
			UID:  1000,
			GID:  1000,
		},
		MkdirOperation{
			Path: path,
			Mode: 0o700,
			UID:  1000,
			GID:  1000,
		},
	)

	err := ValidatePlan(plan)
	if err == nil {
		t.Fatal(
			"ValidatePlan() succeeded, want error",
		)
	}

	assertPlanErrorContains(
		t,
		err,
		"conflicting mkdir configuration",
	)
}

func TestValidatePlanRejectsConflictingMkdirUID(
	t *testing.T,
) {
	root := t.TempDir()

	path := filepath.Join(
		root,
		"output",
	)

	plan := mkdirConflictPlan(
		path,
		MkdirOperation{
			Path: path,
			Mode: 0o755,
			UID:  1000,
			GID:  1000,
		},
		MkdirOperation{
			Path: path,
			Mode: 0o755,
			UID:  2000,
			GID:  1000,
		},
	)

	err := ValidatePlan(plan)
	if err == nil {
		t.Fatal(
			"ValidatePlan() succeeded, want error",
		)
	}

	assertPlanErrorContains(
		t,
		err,
		"conflicting mkdir configuration",
	)
}

func TestValidatePlanRejectsConflictingMkdirGID(
	t *testing.T,
) {
	root := t.TempDir()

	path := filepath.Join(
		root,
		"output",
	)

	plan := mkdirConflictPlan(
		path,
		MkdirOperation{
			Path: path,
			Mode: 0o755,
			UID:  1000,
			GID:  1000,
		},
		MkdirOperation{
			Path: path,
			Mode: 0o755,
			UID:  1000,
			GID:  2000,
		},
	)

	err := ValidatePlan(plan)
	if err == nil {
		t.Fatal(
			"ValidatePlan() succeeded, want error",
		)
	}

	assertPlanErrorContains(
		t,
		err,
		"conflicting mkdir configuration",
	)
}

func TestValidatePlanRejectsNilPlannedAction(
	t *testing.T,
) {
	plan := Plan{
		Actions: []PlannedAction{
			{},
		},
	}

	err := ValidatePlan(plan)
	if err == nil {
		t.Fatal(
			"ValidatePlan() succeeded, want error",
		)
	}

	assertPlanErrorContains(
		t,
		err,
		"planned action 0 has nil action",
	)
}

func TestValidatePlanRejectsConflictingMkdirSymlinkAliases(
	t *testing.T,
) {
	root := t.TempDir()

	realDir := filepath.Join(
		root,
		"real",
	)

	if err := os.MkdirAll(
		realDir,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	linkDir := filepath.Join(
		root,
		"link",
	)

	if err := os.Symlink(
		realDir,
		linkDir,
	); err != nil {
		t.Skipf(
			"cannot create symlink: %v",
			err,
		)
	}

	first := filepath.Join(
		realDir,
		"output",
	)

	second := filepath.Join(
		linkDir,
		"output",
	)

	plan := Plan{
		Actions: []PlannedAction{
			{
				Action: &action.Action{
					ActionFile: "one.yaml",
				},
				PreRun: []PreRunOperation{
					MkdirOperation{
						Path: first,
						Mode: 0o755,
						UID:  1000,
						GID:  1000,
					},
				},
			},
			{
				Action: &action.Action{
					ActionFile: "two.yaml",
				},
				PreRun: []PreRunOperation{
					MkdirOperation{
						Path: second,
						Mode: 0o700,
						UID:  1000,
						GID:  1000,
					},
				},
			},
		},
	}

	err := ValidatePlan(plan)
	if err == nil {
		t.Fatal(
			"ValidatePlan() succeeded, want error",
		)
	}

	assertPlanErrorContains(
		t,
		err,
		"conflicting mkdir configuration",
	)
}

func TestBuildPlanCompilesOnFail(t *testing.T) {
	a := plannedTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    run:
      docker:
        image: primary:latest
        entrypoint:
          - yamr
        cmd_sources:
          - source.yaml

    on_fail:
      run:
        docker:
          image: debug:latest
          entrypoint:
            - yamr
          cmd_sources:
            - source.yaml
`,
	)

	plan, err := BuildPlan(
		[]*action.Action{a},
	)
	if err != nil {
		t.Fatalf(
			"BuildPlan() error = %v",
			err,
		)
	}

	if plan.Actions[0].OnFail == nil {
		t.Fatal(
			"OnFail = nil, want compiled on_fail",
		)
	}

	if plan.Actions[0].OnFail.Docker.Image !=
		"debug:latest" {
		t.Fatalf(
			"OnFail.Docker.Image = %q",
			plan.Actions[0].OnFail.Docker.Image,
		)
	}
}

func plannedTestAction(
	t *testing.T,
	actionFile string,
	value string,
) *action.Action {
	t.Helper()

	a := actionFromYAML(t, value)

	a.ActionFile = actionFile
	a.ActionDir = filepath.Dir(actionFile)

	return a
}

func assertPlanErrorContains(
	t *testing.T,
	err error,
	want string,
) {
	t.Helper()

	if !strings.Contains(
		err.Error(),
		want,
	) {
		t.Fatalf(
			"error = %q, want error containing %q",
			err,
			want,
		)
	}
}

func mkdirConflictPlan(
	path string,
	first MkdirOperation,
	second MkdirOperation,
) Plan {
	first.Path = path
	second.Path = path

	return Plan{
		Actions: []PlannedAction{
			{
				Action: &action.Action{
					ActionFile: "one.yaml",
				},
				PreRun: []PreRunOperation{
					first,
				},
			},
			{
				Action: &action.Action{
					ActionFile: "two.yaml",
				},
				PreRun: []PreRunOperation{
					second,
				},
			},
		},
	}
}
