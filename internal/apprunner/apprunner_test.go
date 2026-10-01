package apprunner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"jinal--shah/yamr-run/internal/app"
	"jinal--shah/yamr-run/internal/cli"
	"jinal--shah/yamr-run/internal/discover"
	"jinal--shah/yamr-run/internal/docker"
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
	if !strings.Contains(harness.stderr.String(), wantStderr) {
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

	// adding this here, rather than an entire test for checking prepare calls
	if harness.prepareCalls != 0 {
		t.Fatalf(
			"prepare calls = %d, want 0",
			harness.prepareCalls,
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

func TestRunPassesPlanAndMaxWorkersToPrepare(
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

	if harness.prepareCalls != 1 {
		t.Fatalf(
			"prepare calls = %d, want 1",
			harness.prepareCalls,
		)
	}

	if len(harness.gotPreparePlan.Actions) != 2 {
		t.Fatalf(
			"prepare plan actions = %d, want 2",
			len(harness.gotPreparePlan.Actions),
		)
	}

	if harness.gotPrepareMaxWorkers != 7 {
		t.Fatalf(
			"prepare max workers = %d, want 7",
			harness.gotPrepareMaxWorkers,
		)
	}
}

func TestRunPrepareErrorDoesNotExecute(
	t *testing.T,
) {
	harness := newRunHarness()

	prepareErr := errors.New(
		"docker unavailable",
	)

	harness.prepareErr = prepareErr

	harness.buildResult = app.BuildResult{
		Plan: runner.Plan{
			Actions: []runner.PlannedAction{
				{},
			},
		},
	}

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
		prepareErr,
	) {
		t.Fatalf(
			"run() error = %v, want wrapped prepare error",
			err,
		)
	}

	if !strings.Contains(
		err.Error(),
		"prepare execution",
	) {
		t.Fatalf(
			"run() error = %q, want prepare execution error",
			err,
		)
	}

	if harness.prepareCalls != 1 {
		t.Fatalf(
			"prepare calls = %d, want 1",
			harness.prepareCalls,
		)
	}

	if harness.executeCalls != 0 {
		t.Fatalf(
			"execute calls = %d, want 0",
			harness.executeCalls,
		)
	}

	if strings.Contains(
		harness.stderr.String(),
		"Completed:",
	) {
		t.Fatalf(
			"stderr = %q, want no execution summary",
			harness.stderr.String(),
		)
	}

	if result.Executed {
		t.Error(
			"Executed = true, want false",
		)
	}
}

func TestPrepareExecutionPullsPlanImages(
	t *testing.T,
) {
	plan := runner.Plan{
		Actions: []runner.PlannedAction{
			{
				Docker: docker.Command{
					Image: "example/a:1",
				},
			},
			{
				Docker: docker.Command{
					Image: "example/b:2",
				},
			},
			{
				Docker: docker.Command{
					Image: "example/a:1",
				},
			},
		},
	}

	checkCalls := 0

	checkDocker := func(
		ctx context.Context,
	) error {
		checkCalls++

		return nil
	}

	var mutex sync.Mutex
	pulled := make(
		map[string]int,
	)

	pullImage := func(
		ctx context.Context,
		image string,
	) error {
		mutex.Lock()
		defer mutex.Unlock()

		pulled[image]++

		return nil
	}

	err := prepareExecution(
		context.Background(),
		plan,
		2,
		checkDocker,
		pullImage,
	)
	if err != nil {
		t.Fatalf(
			"prepareExecution() error = %v, want nil",
			err,
		)
	}

	if checkCalls != 1 {
		t.Fatalf(
			"check Docker calls = %d, want 1",
			checkCalls,
		)
	}

	mutex.Lock()
	defer mutex.Unlock()

	if len(pulled) != 2 {
		t.Fatalf(
			"pulled images = %#v, want 2 distinct images",
			pulled,
		)
	}

	for _, image := range []string{
		"example/a:1",
		"example/b:2",
	} {
		if pulled[image] != 1 {
			t.Errorf(
				"pull count for %q = %d, want 1",
				image,
				pulled[image],
			)
		}
	}
}

func TestPrepareExecutionDockerCheckFailureDoesNotPull(
	t *testing.T,
) {
	checkErr := errors.New(
		"Docker unavailable",
	)

	checkDocker := func(
		ctx context.Context,
	) error {
		return checkErr
	}

	pullCalls := 0

	pullImage := func(
		ctx context.Context,
		image string,
	) error {
		pullCalls++

		return nil
	}

	plan := runner.Plan{
		Actions: []runner.PlannedAction{
			{
				Docker: docker.Command{
					Image: "example/a:1",
				},
			},
		},
	}

	err := prepareExecution(
		context.Background(),
		plan,
		2,
		checkDocker,
		pullImage,
	)

	if !errors.Is(
		err,
		checkErr,
	) {
		t.Fatalf(
			"prepareExecution() error = %v, want Docker check error",
			err,
		)
	}

	if pullCalls != 0 {
		t.Fatalf(
			"pull calls = %d, want 0",
			pullCalls,
		)
	}
}

func TestPrepareExecutionPullFailure(
	t *testing.T,
) {
	pullErr := errors.New(
		"manifest unknown",
	)

	checkDocker := func(
		ctx context.Context,
	) error {
		return nil
	}

	pullImage := func(
		ctx context.Context,
		image string,
	) error {
		if image == "example/b:2" {
			return pullErr
		}

		return nil
	}

	plan := runner.Plan{
		Actions: []runner.PlannedAction{
			{
				Docker: docker.Command{
					Image: "example/a:1",
				},
			},
			{
				Docker: docker.Command{
					Image: "example/b:2",
				},
			},
		},
	}

	err := prepareExecution(
		context.Background(),
		plan,
		2,
		checkDocker,
		pullImage,
	)
	if err == nil {
		t.Fatal(
			"prepareExecution() error = nil, want error",
		)
	}

	if !errors.Is(
		err,
		pullErr,
	) {
		t.Fatalf(
			"prepareExecution() error = %v, want wrapped pull error",
			err,
		)
	}

	if !strings.Contains(
		err.Error(),
		"example/b:2",
	) {
		t.Fatalf(
			"prepareExecution() error = %q, want failing image",
			err,
		)
	}
}

func TestPrepareExecutionRespectsMaxWorkers(
	t *testing.T,
) {
	plan := runner.Plan{
		Actions: []runner.PlannedAction{
			{
				Docker: docker.Command{
					Image: "example/a:1",
				},
			},
			{
				Docker: docker.Command{
					Image: "example/b:1",
				},
			},
			{
				Docker: docker.Command{
					Image: "example/c:1",
				},
			},
			{
				Docker: docker.Command{
					Image: "example/d:1",
				},
			},
		},
	}

	checkDocker := func(
		ctx context.Context,
	) error {
		return nil
	}

	const maxWorkers = 2

	started := make(
		chan string,
		len(plan.Actions),
	)
	release := make(chan struct{})

	pullImage := func(
		ctx context.Context,
		image string,
	) error {
		started <- image

		<-release

		return nil
	}

	type prepareResult struct {
		err error
	}

	done := make(
		chan prepareResult,
		1,
	)

	go func() {
		done <- prepareResult{
			err: prepareExecution(
				context.Background(),
				plan,
				maxWorkers,
				checkDocker,
				pullImage,
			),
		}
	}()

	for range maxWorkers {
		<-started
	}

	// Both workers are now blocked inside pullImage. If the worker
	// bound is respected, no third pull can start.
	select {
	case image := <-started:
		t.Fatalf(
			"pull for %q started while %d workers were already active",
			image,
			maxWorkers,
		)
	default:
	}

	close(
		release,
	)

	got := <-done

	if got.err != nil {
		t.Fatalf(
			"prepareExecution() error = %v, want nil",
			got.err,
		)
	}

	startedCount := maxWorkers

	for {
		select {
		case <-started:
			startedCount++
		default:
			goto counted
		}
	}

counted:
	if startedCount != len(plan.Actions) {
		t.Fatalf(
			"started pulls = %d, want %d",
			startedCount,
			len(plan.Actions),
		)
	}
}

func TestPrepareExecutionContextCancelled(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	checkCalls := 0
	pullCalls := 0

	checkDocker := func(
		ctx context.Context,
	) error {
		checkCalls++

		return ctx.Err()
	}

	pullImage := func(
		ctx context.Context,
		image string,
	) error {
		pullCalls++

		return nil
	}

	plan := runner.Plan{
		Actions: []runner.PlannedAction{
			{
				Docker: docker.Command{
					Image: "example/a:1",
				},
			},
		},
	}

	err := prepareExecution(
		ctx,
		plan,
		2,
		checkDocker,
		pullImage,
	)

	if !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf(
			"prepareExecution() error = %v, want context.Canceled",
			err,
		)
	}

	if pullCalls != 0 {
		t.Fatalf(
			"pull calls = %d, want 0",
			pullCalls,
		)
	}

	if checkCalls > 1 {
		t.Fatalf(
			"check calls = %d, want at most 1",
			checkCalls,
		)
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
		"  ?? bar.go\n"
		//

	if !strings.Contains(harness.stderr.String(), want) {
		t.Fatalf(
			"stderr = %q, should contain %q",
			harness.stderr.String(),
			want,
		)
	}

	want = "Completed: 1 action, 1 succeeded, 0 failed.\n"
	if !strings.Contains(harness.stderr.String(), want) {
		t.Fatalf(
			"stderr = %q, should contain %q",
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
		"(triggered by /repo/foo/.yamr.yaml)\n"

	if !strings.Contains(harness.stderr.String(), want) {
		t.Fatalf(
			"stderr = %q, should contain %q",
			harness.stderr.String(),
			want,
		)
	}
	want = "Completed: 1 action, 1 succeeded, 0 failed.\n"
	if !strings.Contains(harness.stderr.String(), want) {
		t.Fatalf(
			"stderr = %q, should contain %q",
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

	if harness.prepareCalls != 0 {
		t.Fatalf(
			"prepare calls = %d, want 0",
			harness.prepareCalls,
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
		"Docker images ready.\n" +
		"Completed: 3 actions, 3 succeeded, 0 failed.\n"

	if !strings.Contains(harness.stderr.String(), want) {
		t.Fatalf(
			"stderr = %q, should contain %q",
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

	want := "Completed: 3 actions, 2 succeeded, 1 failed.\n"
	if !strings.Contains(harness.stderr.String(), want) {
		t.Fatalf(
			"stderr = %q, should contain %q",
			harness.stderr.String(),
			want,
		)
	}
}

type runHarness struct {
	runner   applicationRunner
	reporter report.Reporter

	buildResult app.BuildResult
	buildErr    error
	buildCalls  int

	gotBuildOptions app.BuildOptions

	prepareErr   error
	prepareCalls int

	gotPreparePlan       runner.Plan
	gotPrepareMaxWorkers int

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

		prepare: func(
			ctx context.Context,
			plan runner.Plan,
			maxWorkers int,
		) error {
			harness.prepareCalls++
			harness.gotPreparePlan = plan
			harness.gotPrepareMaxWorkers = maxWorkers

			return harness.prepareErr
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
		CLI:     cli.Options{},
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
				ActionFile:  "/repo/action/descendant/.yamr.yaml",
				TriggeredBy: "/repo/action/.yamr.yaml",
			},
		},
		Plan: plan,
	}
}
