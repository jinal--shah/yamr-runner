package apprunner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"jinal--shah/yamr-run/internal/app"
	"jinal--shah/yamr-run/internal/cli"
	"jinal--shah/yamr-run/internal/discover"
	"jinal--shah/yamr-run/internal/git"
	"jinal--shah/yamr-run/internal/report"
	"jinal--shah/yamr-run/internal/runner"
)

func TestRunCleanWithoutIgnoredActionsExecutesWithoutPrompt(
	t *testing.T,
) {
	harness := newRunHarness()

	// must have a plan, or run() will report nothing executed
	plan := runner.Plan{
		Actions: []runner.PlannedAction{
			{},
		},
	}

	harness.buildResult = app.BuildResult{
		Repository: git.Repository{
			Root: "/repo",
		},
		Plan: plan,
	}

	// also need to fudge the executeResult, as our executor will
	// not count the empty planned action in its executed
	harness.executeResult = runner.PlanResult{
		Actions: []runner.PlanActionResult{
			{
				Index: 0,
			},
		},
	}

	result, err := harness.run(
		t,
		"",
		false,
	)
	if err != nil {
		t.Fatalf(
			"run() error = %v",
			err,
		)
	}

	if !result.Executed {
		t.Error(
			"Executed = false, want true",
		)
	}

	if result.Declined {
		t.Error(
			"Declined = true, want false",
		)
	}

	if harness.executeCalls != 1 {
		t.Fatalf(
			"execute calls = %d, want 1",
			harness.executeCalls,
		)
	}

	if harness.stdout.String() != "" {
		t.Fatalf(
			"stdout = %q, want empty",
			harness.stdout.String(),
		)
	}

	wantStderr := "Completed: 1 action, 1 succeeded, 0 failed.\n"
	if harness.stderr.String() != wantStderr {
		t.Fatalf(
			"stderr = %q, want %q",
			harness.stderr.String(),
			wantStderr,
		)
	}

	// stderr will always have a summary so no point checking

}

func TestRunDirtyRepositoryExecutesWithNoPrompts(
	t *testing.T,
) {
	harness := newRunHarness()

	plan := runner.Plan{
		Actions: []runner.PlannedAction{
			{},
		},
	}

	harness.buildResult = app.BuildResult{
		Repository: git.Repository{
			Root: "/repo",
		},
		GitStatus: git.Status{
			Dirty: true,
		},
		Plan: plan,
	}

	harness.executeResult = runner.PlanResult{
		Actions: []runner.PlanActionResult{
			{
				Index: 0,
			},
		},
	}

	result, err := harness.run(
		t,
		"",
		true, // --no-prompts
	)
	if err != nil {
		t.Fatalf(
			"run() error = %v",
			err,
		)
	}

	if !result.Executed {
		t.Error(
			"Executed = false, want true",
		)
	}

	if result.Declined {
		t.Error(
			"Declined = true, want false",
		)
	}

	if harness.executeCalls != 1 {
		t.Fatalf(
			"execute calls = %d, want 1",
			harness.executeCalls,
		)
	}

	if !strings.Contains(
		harness.stderr.String(),
		"NOTICE:",
	) {
		t.Fatalf(
			"stderr = %q, want notice",
			harness.stderr.String(),
		)
	}

	if !strings.Contains(
		harness.stderr.String(),
		"uncommitted changes",
	) {
		t.Fatalf(
			"stderr = %q, want dirty repository notice",
			harness.stderr.String(),
		)
	}

	if strings.Contains(
		harness.stderr.String(),
		"Continue?",
	) {
		t.Fatalf(
			"stderr = %q, --no-prompts must not prompt",
			harness.stderr.String(),
		)
	}
}

func TestRunDirtyRepositoryPrompts(
	t *testing.T,
) {
	harness := newRunHarness()

	harness.buildResult = app.BuildResult{
		Repository: git.Repository{
			Root: "/repo",
		},
		GitStatus: git.Status{
			Dirty: true,
		},
		Plan: runner.Plan{
			Actions: []runner.PlannedAction{
				{},
			},
		},
	}

	result, err := harness.run(
		t,
		"n\n",
		false,
	)
	if err != nil {
		t.Fatalf(
			"run() error = %v",
			err,
		)
	}

	if result.Executed {
		t.Error(
			"Executed = true, want false",
		)
	}

	if !result.Declined {
		t.Error(
			"Declined = false, want true",
		)
	}

	if harness.executeCalls != 0 {
		t.Fatalf(
			"execute calls = %d, want 0",
			harness.executeCalls,
		)
	}

	if !strings.Contains(
		harness.stderr.String(),
		"Continue?",
	) {
		t.Fatalf(
			"stderr = %q, want confirmation prompt",
			harness.stderr.String(),
		)
	}
}

func TestRunIgnoredActionsYesExecutes(
	t *testing.T,
) {
	tests := []struct {
		name   string
		answer string
	}{
		{
			name:   "y",
			answer: "y\n",
		},
		{
			name:   "yes",
			answer: "yes\n",
		},
		{
			name:   "uppercase",
			answer: "YES\n",
		},
		{
			name:   "whitespace",
			answer: "  y  \n",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				harness := newRunHarness()

				harness.buildResult =
					buildResultWithIgnoredAction()

				result, err := harness.run(
					t,
					test.answer,
					false,
				)
				if err != nil {
					t.Fatalf(
						"run() error = %v",
						err,
					)
				}

				if !result.Executed {
					t.Error(
						"Executed = false, want true",
					)
				}

				if result.Declined {
					t.Error(
						"Declined = true, want false",
					)
				}

				if harness.executeCalls != 1 {
					t.Fatalf(
						"execute calls = %d, want 1",
						harness.executeCalls,
					)
				}

				if !strings.Contains(
					harness.stderr.String(),
					"NOTICE",
				) {
					t.Fatalf(
						"stderr = %q, want NOTICE",
						harness.stderr.String(),
					)
				}

				if !strings.Contains(
					harness.stderr.String(),
					"/repo/action/descendant/.yamr.yaml",
				) {
					t.Fatalf(
						"stderr = %q, want ignored action path",
						harness.stderr.String(),
					)
				}

				if !strings.Contains(
					harness.stderr.String(),
					"Continue?",
				) {
					t.Fatalf(
						"stderr = %q, want confirmation prompt",
						harness.stderr.String(),
					)
				}
			},
		)
	}
}

func TestRunIgnoredActionsDeclinedDoesNotExecute(
	t *testing.T,
) {
	tests := []struct {
		name   string
		answer string
	}{
		{
			name:   "n",
			answer: "n\n",
		},
		{
			name:   "no",
			answer: "no\n",
		},
		{
			name:   "empty",
			answer: "\n",
		},
		{
			name:   "anything else",
			answer: "maybe\n",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				harness := newRunHarness()

				harness.buildResult =
					buildResultWithIgnoredAction()

				result, err := harness.run(
					t,
					test.answer,
					false,
				)
				if err != nil {
					t.Fatalf(
						"run() error = %v",
						err,
					)
				}

				if result.Executed {
					t.Error(
						"Executed = true, want false",
					)
				}

				if !result.Declined {
					t.Error(
						"Declined = false, want true",
					)
				}

				if harness.executeCalls != 0 {
					t.Fatalf(
						"execute calls = %d, want 0",
						harness.executeCalls,
					)
				}

				if !strings.Contains(
					harness.stderr.String(),
					"NOTICE",
				) {
					t.Fatalf(
						"stderr = %q, want NOTICE",
						harness.stderr.String(),
					)
				}

				if !strings.Contains(
					harness.stderr.String(),
					"Continue?",
				) {
					t.Fatalf(
						"stderr = %q, want confirmation prompt",
						harness.stderr.String(),
					)
				}
			},
		)
	}
}

func TestRunIgnoredActionsEOFDeclines(
	t *testing.T,
) {
	harness := newRunHarness()

	harness.buildResult =
		buildResultWithIgnoredAction()

	result, err := harness.run(
		t,
		"",
		false,
	)
	if err != nil {
		t.Fatalf(
			"run() error = %v",
			err,
		)
	}

	if result.Executed {
		t.Error(
			"Executed = true, want false",
		)
	}

	if !result.Declined {
		t.Error(
			"Declined = false, want true",
		)
	}

	if harness.executeCalls != 0 {
		t.Fatalf(
			"execute calls = %d, want 0",
			harness.executeCalls,
		)
	}
}

func TestRunNoPromptsNoticesAndExecutesIgnoredActions(
	t *testing.T,
) {
	harness := newRunHarness()

	harness.buildResult =
		buildResultWithIgnoredAction()

	result, err := harness.run(
		t,
		"",
		true,
	)
	if err != nil {
		t.Fatalf(
			"run() error = %v",
			err,
		)
	}

	if !result.Executed {
		t.Error(
			"Executed = false, want true",
		)
	}

	if result.Declined {
		t.Error(
			"Declined = true, want false",
		)
	}

	if harness.executeCalls != 1 {
		t.Fatalf(
			"execute calls = %d, want 1",
			harness.executeCalls,
		)
	}

	if !strings.Contains(
		harness.stderr.String(),
		"NOTICE",
	) {
		t.Fatalf(
			"stderr = %q, want NOTICE",
			harness.stderr.String(),
		)
	}

	if !strings.Contains(
		harness.stderr.String(),
		"/repo/action/descendant/.yamr.yaml",
	) {
		t.Fatalf(
			"stderr = %q, want ignored action path",
			harness.stderr.String(),
		)
	}

	if strings.Contains(
		harness.stderr.String(),
		"Continue?",
	) {
		t.Fatalf(
			"stderr = %q, --no-prompts must not prompt",
			harness.stderr.String(),
		)
	}
}

func TestRunDirtyAndIgnoredWithNoPromptsNoticesSoExecutes(
	t *testing.T,
) {
	harness := newRunHarness()

	harness.buildResult =
		buildResultWithIgnoredAction()
	harness.buildResult.GitStatus.Dirty = true

	result, err := harness.run(
		t,
		"",
		true,
	)
	if err != nil {
		t.Fatalf(
			"run() error = %v",
			err,
		)
	}

	if !result.Executed {
		t.Error(
			"Executed = false, want true",
		)
	}

	if result.Declined {
		t.Error(
			"Declined = true, want false",
		)
	}

	if harness.executeCalls != 1 {
		t.Fatalf(
			"execute calls = %d, want 1",
			harness.executeCalls,
		)
	}

	stderr := harness.stderr.String()

	if !strings.Contains(
		stderr,
		"uncommitted changes",
	) {
		t.Fatalf(
			"stderr = %q, want dirty repository warning",
			stderr,
		)
	}

	if !strings.Contains(
		stderr,
		"/repo/action/descendant/.yamr.yaml",
	) {
		t.Fatalf(
			"stderr = %q, want ignored action warning",
			stderr,
		)
	}

	if strings.Contains(
		harness.stderr.String(),
		"Continue?",
	) {
		t.Fatalf(
			"stderr = %q, --no-prompts must not prompt",
			harness.stderr.String(),
		)
	}
}

func TestRunBuildErrorDoesNotExecute(
	t *testing.T,
) {
	harness := newRunHarness()

	buildErr := errors.New(
		"build failed",
	)

	harness.buildErr = buildErr

	result, err := harness.run(
		t,
		"",
		false,
	)

	if err == nil {
		t.Fatal(
			"run() error = nil, want error",
		)
	}

	if !errors.Is(
		err,
		buildErr,
	) {
		t.Fatalf(
			"run() error = %v, want wrapped build error",
			err,
		)
	}

	if result.Executed {
		t.Error(
			"Executed = true, want false",
		)
	}

	if result.Declined {
		t.Error(
			"Declined = true, want false",
		)
	}

	if harness.executeCalls != 0 {
		t.Fatalf(
			"execute calls = %d, want 0",
			harness.executeCalls,
		)
	}
}

func TestRunExecutionErrorReturnsExecutedResult(
	t *testing.T,
) {
	harness := newRunHarness()

	// must have a plan, or run() will report nothing executed
	plan := runner.Plan{
		Actions: []runner.PlannedAction{
			{},
		},
	}

	harness.buildResult = app.BuildResult{
		Plan: plan,
	}

	executionErr := errors.New(
		"execution failed",
	)

	harness.executeErr = executionErr

	result, err := harness.run(
		t,
		"",
		false,
	)

	if err == nil {
		t.Fatal(
			"run() error = nil, want error",
		)
	}

	if !errors.Is(
		err,
		executionErr,
	) {
		t.Fatalf(
			"run() error = %v, want wrapped execution error",
			err,
		)
	}

	if !result.Executed {
		t.Error(
			"Executed = false, want true",
		)
	}

	if result.Declined {
		t.Error(
			"Declined = true, want false",
		)
	}

	if harness.executeCalls != 1 {
		t.Fatalf(
			"execute calls = %d, want 1",
			harness.executeCalls,
		)
	}
}

func TestRunPassesBuildOptionsToBuilder(
	t *testing.T,
) {
	harness := newRunHarness()

	hostEnv := []string{
		"FOO=bar",
		"BAZ=qux",
	}

	cliOptions := cli.Options{
		ConfigFile:       "/config.yaml",
		YamrSourcesDir:   "/sources",
		YamrSourceLabels: "/labels.yaml",
		NoPrompts:        true,
	}

	options := harness.options(
		"",
	)
	options.CLI = cliOptions
	options.RunDir = "/run"
	options.HostEnv = hostEnv

	_, err := harness.runner.run(
		context.Background(),
		options,
		harness.reporter,
	)
	if err != nil {
		t.Fatalf(
			"run() error = %v",
			err,
		)
	}

	if harness.buildCalls != 1 {
		t.Fatalf(
			"build calls = %d, want 1",
			harness.buildCalls,
		)
	}

	if harness.gotBuildOptions.RunDir != "/run" {
		t.Fatalf(
			"build RunDir = %q, want /run",
			harness.gotBuildOptions.RunDir,
		)
	}

	if harness.gotBuildOptions.CLI != cliOptions {
		t.Fatalf(
			"build CLI = %#v, want %#v",
			harness.gotBuildOptions.CLI,
			cliOptions,
		)
	}

	if len(harness.gotBuildOptions.HostEnv) !=
		len(hostEnv) {
		t.Fatalf(
			"build HostEnv = %#v, want %#v",
			harness.gotBuildOptions.HostEnv,
			hostEnv,
		)
	}

	for i := range hostEnv {
		if harness.gotBuildOptions.HostEnv[i] !=
			hostEnv[i] {
			t.Fatalf(
				"build HostEnv = %#v, want %#v",
				harness.gotBuildOptions.HostEnv,
				hostEnv,
			)
		}
	}
}

func TestRunPassesPlanAndMaxWorkersToExecutor(
	t *testing.T,
) {
	harness := newRunHarness()

	plan := runner.Plan{
		Actions: []runner.PlannedAction{
			{},
			{},
		},
	}

	harness.buildResult = app.BuildResult{
		Plan: plan,
	}

	options := harness.options(
		"",
	)
	options.CLI.MaxWorkers = 7

	_, err := harness.runner.run(
		context.Background(),
		options,
		harness.reporter,
	)
	if err != nil {
		t.Fatalf(
			"run() error = %v",
			err,
		)
	}

	if harness.executeCalls != 1 {
		t.Fatalf(
			"execute calls = %d, want 1",
			harness.executeCalls,
		)
	}

	if len(harness.gotPlan.Actions) != 2 {
		t.Fatalf(
			"executor plan actions = %d, want 2",
			len(harness.gotPlan.Actions),
		)
	}

	if harness.gotMaxWorkers != 7 {
		t.Fatalf(
			"executor max workers = %d, want 7",
			harness.gotMaxWorkers,
		)
	}
}

func TestRunReportsDirtyGit(
	t *testing.T,
) {
	harness := newRunHarness()

	harness.buildResult = app.BuildResult{
		Repository: git.Repository{
			Root: "/repo",
		},
		GitStatus: git.Status{
			Dirty: true,
			Lines: []string{
				" M foo.go",
				"?? bar.go",
			},
		},
		Plan: runner.Plan{
			Actions: []runner.PlannedAction{
				{},
			},
		},
	}

	harness.executeResult = runner.PlanResult{
		Actions: []runner.PlanActionResult{
			{
				Index: 0,
			},
		},
	}

	_, err := harness.run(
		t,
		"",
		true,
	)
	if err != nil {
		t.Fatalf(
			"run() error = %v",
			err,
		)
	}

	want := "" +
		"NOTICE: Git repository /repo has uncommitted changes\n" +
		"   M foo.go\n" +
		"  ?? bar.go\n" +
		"Completed: 1 action, 1 succeeded, 0 failed.\n"

	if harness.stderr.String() != want {
		t.Fatalf(
			"stderr = %q, want %q",
			harness.stderr.String(),
			want,
		)
	}
}

func TestRunReportsIgnoredActionFiles(
	t *testing.T,
) {
	harness := newRunHarness()

	harness.buildResult = app.BuildResult{
		Plan: runner.Plan{
			Actions: []runner.PlannedAction{
				{},
			},
		},
		IgnoredActionFiles: []discover.IgnoredActionFile{
			{
				ActionFile:  "/repo/foo/bar/.yamr.yaml",
				TriggeredBy: "/repo/foo/.yamr.yaml",
			},
			{
				ActionFile:  "/repo/foo/baz/.yamr.yaml",
				TriggeredBy: "/repo/foo/.yamr.yaml",
			},
		},
	}

	harness.executeResult = runner.PlanResult{
		Actions: []runner.PlanActionResult{
			{
				Index: 0,
			},
		},
	}

	_, err := harness.run(
		t,
		"",
		true,
	)
	if err != nil {
		t.Fatalf(
			"run() error = %v",
			err,
		)
	}

	want := "" +
		"NOTICE: 2 action file(s) will not run because " +
		"an earlier action on their path was triggered:\n" +
		"  /repo/foo/bar/.yamr.yaml " +
		"(triggered by /repo/foo/.yamr.yaml)\n" +
		"  /repo/foo/baz/.yamr.yaml " +
		"(triggered by /repo/foo/.yamr.yaml)\n" +
		"Completed: 1 action, 1 succeeded, 0 failed.\n"

	if harness.stderr.String() != want {
		t.Fatalf(
			"stderr = %q, want %q",
			harness.stderr.String(),
			want,
		)
	}
}

func TestRunNoActionsReportsAndDoesNotPrompt(
	t *testing.T,
) {
	harness := newRunHarness()

	harness.buildResult = app.BuildResult{
		Repository: git.Repository{
			Root: "/repo",
		},
		GitStatus: git.Status{
			Dirty: true,
		},
	}

	result, err := harness.run(
		t,
		"",
		false,
	)
	if err != nil {
		t.Fatalf(
			"run() error = %v",
			err,
		)
	}

	if result.Executed {
		t.Error(
			"Executed = true, want false",
		)
	}

	if result.Declined {
		t.Error(
			"Declined = true, want false",
		)
	}

	if harness.executeCalls != 0 {
		t.Fatalf(
			"execute calls = %d, want 0",
			harness.executeCalls,
		)
	}

	if harness.stdout.String() != "" {
		t.Fatalf(
			"stdout = %q, want empty",
			harness.stdout.String(),
		)
	}

	wantStderr := "" +
		"NOTICE: Git repository /repo has uncommitted changes\n" +
		"No triggered actions found.\n"

	if harness.stderr.String() != wantStderr {
		t.Fatalf(
			"stderr = %q, want %q",
			harness.stderr.String(),
			wantStderr,
		)
	}
}

func TestRunReportsSummaryAfterSuccess(
	t *testing.T,
) {
	harness := newRunHarness()

	harness.buildResult = app.BuildResult{
		Plan: runner.Plan{
			Actions: []runner.PlannedAction{
				{},
				{},
				{},
			},
		},
	}

	harness.executeResult = runner.PlanResult{
		Actions: []runner.PlanActionResult{
			{
				Index: 0,
			},
			{
				Index: 1,
			},
			{
				Index: 2,
			},
		},
	}

	result, err := harness.run(
		t,
		"",
		false,
	)
	if err != nil {
		t.Fatalf(
			"run() error = %v",
			err,
		)
	}

	if !result.Executed {
		t.Error(
			"Executed = false, want true",
		)
	}

	want := "" +
		"Completed: 3 actions, 3 succeeded, 0 failed.\n"

	if harness.stderr.String() != want {
		t.Fatalf(
			"stderr = %q, want %q",
			harness.stderr.String(),
			want,
		)
	}
}

func TestRunReportsSummaryAfterFailure(
	t *testing.T,
) {
	harness := newRunHarness()

	harness.buildResult = app.BuildResult{
		Plan: runner.Plan{
			Actions: []runner.PlannedAction{
				{},
				{},
				{},
			},
		},
	}

	actionErr := errors.New(
		"docker action failed",
	)

	harness.executeResult = runner.PlanResult{
		Actions: []runner.PlanActionResult{
			{
				Index: 0,
			},
			{
				Index: 1,
				Error: actionErr,
			},
			{
				Index: 2,
			},
		},
	}

	harness.executeErr = errors.New(
		"one or more actions failed",
	)

	result, err := harness.run(
		t,
		"",
		false,
	)
	if err == nil {
		t.Fatal(
			"run() succeeded, want error",
		)
	}

	if !result.Executed {
		t.Error(
			"Executed = false, want true",
		)
	}

	if !strings.Contains(
		err.Error(),
		"execute plan",
	) {
		t.Fatalf(
			"error = %q, want execute plan error",
			err,
		)
	}

	want := "" +
		"Completed: 3 actions, 2 succeeded, 1 failed.\n"

	if harness.stderr.String() != want {
		t.Fatalf(
			"stderr = %q, want %q",
			harness.stderr.String(),
			want,
		)
	}
}

type runHarness struct {
	runner applicationRunner
	reporter report.Reporter

	buildResult app.BuildResult
	buildErr    error
	buildCalls  int

	gotBuildOptions app.BuildOptions

	executeResult runner.PlanResult
	executeErr    error
	executeCalls  int

	gotPlan       runner.Plan
	gotMaxWorkers int

	stdout bytes.Buffer
	stderr bytes.Buffer
}

func newRunHarness() *runHarness {
	harness := &runHarness{}

	harness.reporter = report.NewTextReporter(
		&harness.stderr,
	)

	harness.runner = applicationRunner{
		build: func(
			ctx context.Context,
			options app.BuildOptions,
		) (app.BuildResult, error) {
			harness.buildCalls++
			harness.gotBuildOptions = options

			return harness.buildResult,
				harness.buildErr
		},

		execute: func(
			ctx context.Context,
			plan runner.Plan,
			maxWorkers int,
			stdout io.Writer,
			stderr io.Writer,
		) (runner.PlanResult, error) {
			harness.executeCalls++
			harness.gotPlan = plan
			harness.gotMaxWorkers = maxWorkers

			return harness.executeResult,
				harness.executeErr
		},
	}

	return harness
}

func (h *runHarness) options(
	stdin string,
) Options {
	return Options{
		CLI: cli.Options{},
		RunDir:  "/repo",
		Stdin:   strings.NewReader(stdin),
		Stdout:  &h.stdout,
		Stderr:  &h.stderr,
		HostEnv: []string{},
	}
}

func (h *runHarness) run(
	t *testing.T,
	stdin string,
	noPrompts bool,
) (Result, error) {
	t.Helper()

	options := h.options(
		stdin,
	)
	options.CLI.NoPrompts = noPrompts

	return h.runner.run(
		context.Background(),
		options,
		h.reporter,
	)
}

func buildResultWithIgnoredAction() app.BuildResult {
	// must have a plan, or run() will report nothing executed
	plan := runner.Plan{
		Actions: []runner.PlannedAction{
			{},
		},
	}

	return app.BuildResult{
		Repository: git.Repository{
			Root: "/repo",
		},
		IgnoredActionFiles: []discover.IgnoredActionFile{
			{
				ActionFile: "/repo/action/descendant/.yamr.yaml",
				TriggeredBy: "/repo/action/.yamr.yaml",
			},
		},
		Plan: plan,
	}
}
