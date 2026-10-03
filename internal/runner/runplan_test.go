package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"jinal--shah/yamr-runner/internal/action"
	"jinal--shah/yamr-runner/internal/docker"
)

func TestRunPlanRespectsWorkerLimit(
	t *testing.T,
) {
	var mutex sync.Mutex

	active := 0
	maxActive := 0

	release := make(chan struct{})

	execution := &Execution{
		runDocker: func(
			ctx context.Context,
			command docker.Command,
			stdout io.Writer,
			stderr io.Writer,
		) (docker.Result, error) {
			mutex.Lock()

			active++

			if active > maxActive {
				maxActive = active
			}

			mutex.Unlock()

			select {
			case <-release:
			case <-ctx.Done():
				return docker.Result{}, ctx.Err()
			}

			mutex.Lock()
			active--
			mutex.Unlock()

			return docker.Result{
				ExitCode: 0,
				Ran:      true,
			}, nil
		},
		events: nil,
	}

	plan := testExecutionPlan(
		t,
		10,
	)

	done := make(chan struct{})

	var result PlanResult
	var runErr error

	go func() {
		result, runErr = execution.RunPlan(
			context.Background(),
			plan,
			3,
			nil,
			nil,
		)

		close(done)
	}()

	waitForActiveActions(
		t,
		&mutex,
		&active,
		3,
	)

	mutex.Lock()
	gotMaxActive := maxActive
	mutex.Unlock()

	if gotMaxActive != 3 {
		t.Fatalf(
			"max active actions = %d, want 3",
			gotMaxActive,
		)
	}

	close(release)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal(
			"RunPlan() did not finish",
		)
	}

	if runErr != nil {
		t.Fatalf(
			"RunPlan() error = %v",
			runErr,
		)
	}

	if len(result.Actions) != 10 {
		t.Fatalf(
			"len(Actions) = %d, want 10",
			len(result.Actions),
		)
	}
}

func TestRunPlanContinuesAfterActionFailure(
	t *testing.T,
) {
	var mutex sync.Mutex

	executed := make(
		map[string]int,
	)

	execution := &Execution{
		runDocker: func(
			ctx context.Context,
			command docker.Command,
			stdout io.Writer,
			stderr io.Writer,
		) (docker.Result, error) {
			mutex.Lock()
			executed[command.Image]++
			mutex.Unlock()

			if command.Image == "image-2" {
				return docker.Result{
						ExitCode: 42,
						Ran:      true,
					},
					fmt.Errorf(
						"docker exited with status 42",
					)
			}

			return docker.Result{
				ExitCode: 0,
				Ran:      true,
			}, nil
		},
		events: nil,
	}

	plan := testExecutionPlan(
		t,
		5,
	)

	result, err := execution.RunPlan(
		context.Background(),
		plan,
		2,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf(
			"RunPlan() error = %v",
			err,
		)
	}

	if !result.Failed() {
		t.Fatal(
			"PlanResult.Failed() = false, want true",
		)
	}

	for i := range 5 {
		image := fmt.Sprintf(
			"image-%d",
			i,
		)

		mutex.Lock()
		count := executed[image]
		mutex.Unlock()

		if count != 1 {
			t.Fatalf(
				"%s executed %d times, want 1",
				image,
				count,
			)
		}
	}

	if result.Actions[2].Error == nil {
		t.Fatal(
			"action 2 error = nil, want failure",
		)
	}

	for i := range result.Actions {
		if i == 2 {
			continue
		}

		if result.Actions[i].Error != nil {
			t.Fatalf(
				"action %d error = %v",
				i,
				result.Actions[i].Error,
			)
		}
	}
}

func TestRunPlanPreservesPlanOrder(
	t *testing.T,
) {
	execution := &Execution{
		runDocker: func(
			ctx context.Context,
			command docker.Command,
			stdout io.Writer,
			stderr io.Writer,
		) (docker.Result, error) {
			switch command.Image {
			case "image-0":
				time.Sleep(100 * time.Millisecond)

			case "image-1":
				time.Sleep(50 * time.Millisecond)
			}

			return docker.Result{}, nil
		},
		events: nil,
	}

	plan := testExecutionPlan(
		t,
		3,
	)

	result, err := execution.RunPlan(
		context.Background(),
		plan,
		3,
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	for i, actionResult := range result.Actions {
		if actionResult.Index != i {
			t.Fatalf(
				"Actions[%d].Index = %d",
				i,
				actionResult.Index,
			)
		}

		if actionResult.Result.Action.Action !=
			plan.Actions[i].Action {
			t.Fatalf(
				"Actions[%d] contains wrong action",
				i,
			)
		}
	}
}

func TestPlanResultSummary(
	t *testing.T,
) {
	result := PlanResult{
		Actions: []PlanActionResult{
			{
				Index: 0,
				State: PlanActionRan,
			},
			{
				Index: 1,
				State: PlanActionRan,
				Error: errors.New(
					"action failed",
				),
			},
			{
				Index: 2,
				State: PlanActionRan,
				Error: context.Canceled,
			},
			{
				Index: 3,
				State: PlanActionNotRun,
			},
			{
				Index: 4,
				State: PlanActionNotRun,
			},
		},
	}

	got := result.Summary()

	want := PlanResultSummary{
		Total:     5,
		Ran:       3,
		Succeeded: 1,
		Failed:    2,
		Cancelled: 1,
		NotRun:    2,
	}

	if got != want {
		t.Fatalf(
			"Summary() = %+v, want %+v",
			got,
			want,
		)
	}

	if got.Total != got.Ran+got.NotRun {
		t.Fatalf(
			"Total = %d, Ran + NotRun = %d",
			got.Total,
			got.Ran+got.NotRun,
		)
	}

	if got.Ran != got.Succeeded+got.Failed {
		t.Fatalf(
			"Ran = %d, Succeeded + Failed = %d",
			got.Ran,
			got.Succeeded+got.Failed,
		)
	}
}

func TestPlanResultSummaryRecognizesWrappedCancellation(
	t *testing.T,
) {
	result := PlanResult{
		Actions: []PlanActionResult{
			{
				Index: 0,
				State: PlanActionRan,
				Error: fmt.Errorf(
					"run action: %w",
					context.Canceled,
				),
			},
		},
	}

	got := result.Summary()

	if got.Total != 1 {
		t.Fatalf(
			"Total = %d, want 1",
			got.Total,
		)
	}

	if got.Ran != 1 {
		t.Fatalf(
			"Ran = %d, want 1",
			got.Ran,
		)
	}

	if got.Failed != 1 {
		t.Fatalf(
			"Failed = %d, want 1",
			got.Failed,
		)
	}

	if got.Cancelled != 1 {
		t.Fatalf(
			"Cancelled = %d, want 1",
			got.Cancelled,
		)
	}

	if got.NotRun != 0 {
		t.Fatalf(
			"NotRun = %d, want 0",
			got.NotRun,
		)
	}
}

func TestPlanResultNotRunDoesNotMeanFailed(
	t *testing.T,
) {
	result := PlanResult{
		Actions: []PlanActionResult{
			{
				Index: 0,
				State: PlanActionNotRun,
			},
		},
	}

	if result.Failed() {
		t.Fatal(
			"Failed() = true, want false",
		)
	}

	summary := result.Summary()

	if summary.NotRun != 1 {
		t.Fatalf(
			"NotRun = %d, want 1",
			summary.NotRun,
		)
	}

	if summary.Failed != 0 {
		t.Fatalf(
			"Failed = %d, want 0",
			summary.Failed,
		)
	}
}

func TestRunPlanResultContainsEveryPlannedAction(
	t *testing.T,
) {

	plan := testExecutionPlan(
		t,
		3,
	)

	execution := NewExecution(nil)
	execution.runDocker = func(
		ctx context.Context,
		command docker.Command,
		stdout io.Writer,
		stderr io.Writer,
	) (docker.Result, error) {
		return docker.Result{
			Ran:      true,
			ExitCode: 0,
		}, nil
	}

	result, err := execution.RunPlan(
		context.Background(),
		plan,
		2,
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf(
			"RunPlan() error = %v",
			err,
		)
	}

	if len(result.Actions) != 3 {
		t.Fatalf(
			"len(Actions) = %d, want 3",
			len(result.Actions),
		)
	}

	for index, action := range result.Actions {
		if action.Index != index {
			t.Errorf(
				"Actions[%d].Index = %d, want %d",
				index,
				action.Index,
				index,
			)
		}

		if action.State != PlanActionRan {
			t.Errorf(
				"Actions[%d].State = %q, want %q",
				index,
				action.State,
				PlanActionRan,
			)
		}
	}
}

func TestRunPlanCancellationRecordsRanAndNotRunActions(
	t *testing.T,
) {

	plan := testExecutionPlan(
		t,
		3,
	)
	started := make(chan struct{})
	release := make(chan struct{})

	var startedOnce sync.Once

	execution := NewExecution(nil)
	execution.runDocker = func(
		ctx context.Context,
		command docker.Command,
		stdout io.Writer,
		stderr io.Writer,
	) (docker.Result, error) {
		startedOnce.Do(
			func() {
				close(started)
			},
		)

		<-release

		if err := ctx.Err(); err != nil {
			return docker.Result{}, err
		}

		return docker.Result{
			Ran:      true,
			ExitCode: 0,
		}, nil
	}

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	defer cancel()

	type runResult struct {
		result PlanResult
		err    error
	}

	done := make(chan runResult, 1)

	go func() {
		result, err := execution.RunPlan(
			ctx,
			plan,
			1,
			io.Discard,
			io.Discard,
		)

		done <- runResult{
			result: result,
			err:    err,
		}
	}()

	<-started

	cancel()
	close(release)

	got := <-done

	if !errors.Is(
		got.err,
		context.Canceled,
	) {
		t.Fatalf(
			"RunPlan() error = %v, want context.Canceled",
			got.err,
		)
	}

	summary := got.result.Summary()

	want := PlanResultSummary{
		Total:     3,
		Ran:       1,
		Succeeded: 0,
		Failed:    1,
		Cancelled: 1,
		NotRun:    2,
	}

	if summary != want {
		t.Fatalf(
			"Summary() = %+v, want %+v",
			summary,
			want,
		)
	}

	if got.result.Actions[0].State != PlanActionRan {
		t.Fatalf(
			"Actions[0].State = %q, want %q",
			got.result.Actions[0].State,
			PlanActionRan,
		)
	}

	for _, index := range []int{1, 2} {
		if got.result.Actions[index].State !=
			PlanActionNotRun {
			t.Errorf(
				"Actions[%d].State = %q, want %q",
				index,
				got.result.Actions[index].State,
				PlanActionNotRun,
			)
		}
	}

	wantFileNames := []string{
		"action-0.yaml",
		"action-1.yaml",
		"action-2.yaml",
	}

	for index, want := range wantFileNames {
		got := got.result.Actions[index].
			Result.
			Action.
			Action.
			ActionFile

		if filepath.Base(got) != want {
			t.Errorf(
				"Actions[%d] action file = %q, want %q",
				index,
				got,
				want,
			)
		}
	}
}

func TestRunPlanCancellationDuringOnFailClassifiesActions(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	defer cancel()

	onFailStarted := make(chan struct{})

	execution := NewExecution(nil)

	dockerCalls := 0

	execution.runDocker = func(
		ctx context.Context,
		command docker.Command,
		stdout io.Writer,
		stderr io.Writer,
	) (docker.Result, error) {
		dockerCalls++

		switch dockerCalls {
		case 1:
			// Primary action fails normally.
			return docker.Result{
				Ran:      true,
				ExitCode: 1,
			}, nil

		case 2:
			// on_fail has now actually entered its Docker invocation.
			close(onFailStarted)

			<-ctx.Done()

			return docker.Result{},
				ctx.Err()

		default:
			t.Fatalf(
				"unexpected Docker call %d",
				dockerCalls,
			)

			return docker.Result{}, nil
		}
	}

	firstActionFile := "/repo/a/.yamr.yaml"
	secondActionFile := "/repo/b/.yamr.yaml"

	plan := Plan{
		Actions: []PlannedAction{
			{
				Action: &action.Action{
					ActionFile: firstActionFile,
					ActionDir:  t.TempDir(),
				},
				Docker: docker.Command{
					Image: "jinal--shah/yamr:primary",
				},
				OnFail: &PlannedOnFail{
					Docker: docker.Command{
						Image: "jinal--shah/yamr:on-fail",
					},
					Stdout: OutputDestination{
						Inherit: true,
					},
					Stderr: OutputDestination{
						Inherit: true,
					},
				},
			},
			{
				Action: &action.Action{
					ActionFile: secondActionFile,
					ActionDir:  t.TempDir(),
				},
				Docker: docker.Command{
					Image: "jinal--shah/yamr:second",
				},
			},
		},
	}

	type runResult struct {
		result PlanResult
		err    error
	}

	done := make(
		chan runResult,
		1,
	)

	go func() {
		result, err := execution.RunPlan(
			ctx,
			plan,
			1,
			io.Discard,
			io.Discard,
		)

		done <- runResult{
			result: result,
			err:    err,
		}
	}()

	select {
	case <-onFailStarted:
		// Expected.

	case <-time.After(
		time.Second,
	):
		t.Fatal(
			"timed out waiting for on_fail Docker invocation",
		)
	}

	cancel()

	var got runResult

	select {
	case got = <-done:
		// Expected.

	case <-time.After(
		time.Second,
	):
		t.Fatal(
			"timed out waiting for RunPlan()",
		)
	}

	if !errors.Is(
		got.err,
		context.Canceled,
	) {
		t.Fatalf(
			"RunPlan() error = %v, want context.Canceled",
			got.err,
		)
	}

	if len(got.result.Actions) != 2 {
		t.Fatalf(
			"len(Actions) = %d, want 2",
			len(got.result.Actions),
		)
	}

	first := got.result.Actions[0]

	if first.State != PlanActionRan {
		t.Errorf(
			"Actions[0].State = %q, want %q",
			first.State,
			PlanActionRan,
		)
	}

	if !errors.Is(
		first.Error,
		context.Canceled,
	) {
		t.Errorf(
			"Actions[0].Error = %v, want context.Canceled",
			first.Error,
		)
	}

	if first.Result.Docker.ExitCode != 1 {
		t.Errorf(
			"Actions[0] primary exit code = %d, want 1",
			first.Result.Docker.ExitCode,
		)
	}

	if first.Result.OnFail == nil {
		t.Fatal(
			"Actions[0].Result.OnFail = nil, want result",
		)
	}

	if !errors.Is(
		first.Result.OnFail.Error,
		context.Canceled,
	) {
		t.Errorf(
			"Actions[0] OnFail.Error = %v, want context.Canceled",
			first.Result.OnFail.Error,
		)
	}

	second := got.result.Actions[1]

	if second.State != PlanActionNotRun {
		t.Errorf(
			"Actions[1].State = %q, want %q",
			second.State,
			PlanActionNotRun,
		)
	}

	if second.Error != nil {
		t.Errorf(
			"Actions[1].Error = %v, want nil",
			second.Error,
		)
	}

	summary := got.result.Summary()

	wantSummary := PlanResultSummary{
		Total:     2,
		Ran:       1,
		Succeeded: 0,
		Failed:    1,
		Cancelled: 1,
		NotRun:    1,
	}

	if summary != wantSummary {
		t.Fatalf(
			"Summary() = %+v, want %+v",
			summary,
			wantSummary,
		)
	}

	wantActionFiles := []string{
		firstActionFile,
		secondActionFile,
	}

	for index, want := range wantActionFiles {
		gotActionFile := got.result.Actions[index].
			Result.
			Action.
			Action.
			ActionFile

		if gotActionFile != want {
			t.Errorf(
				"Actions[%d] action file = %q, want %q",
				index,
				gotActionFile,
				want,
			)
		}
	}

	if dockerCalls != 2 {
		t.Errorf(
			"docker calls = %d, want 2",
			dockerCalls,
		)
	}
}

func waitForActiveActions(
	t *testing.T,
	mutex *sync.Mutex,
	active *int,
	want int,
) {
	t.Helper()

	deadline := time.Now().Add(
		5 * time.Second,
	)

	for time.Now().Before(deadline) {
		mutex.Lock()
		got := *active
		mutex.Unlock()

		if got == want {
			return
		}

		time.Sleep(
			10 * time.Millisecond,
		)
	}

	mutex.Lock()
	got := *active
	mutex.Unlock()

	t.Fatalf(
		"active actions = %d, want %d",
		got,
		want,
	)
}

func testExecutionPlan(
	t *testing.T,
	count int,
) Plan {
	t.Helper()

	plan := Plan{
		Actions: make(
			[]PlannedAction,
			0,
			count,
		),
	}

	for i := range count {
		actionFile := filepath.Join(
			t.TempDir(),
			fmt.Sprintf(
				"action-%d.yaml",
				i,
			),
		)

		plan.Actions = append(
			plan.Actions,
			PlannedAction{
				Action: &action.Action{
					ActionFile: actionFile,
					ActionDir: filepath.Dir(
						actionFile,
					),
				},
				Docker: docker.Command{
					Image: fmt.Sprintf(
						"image-%d",
						i,
					),
				},
			},
		)
	}

	return plan
}
