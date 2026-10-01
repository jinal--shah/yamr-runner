package report

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"jinal--shah/yamr-run/internal/action"
	"jinal--shah/yamr-run/internal/discover"
	"jinal--shah/yamr-run/internal/docker"
	"jinal--shah/yamr-run/internal/git"
	"jinal--shah/yamr-run/internal/runner"
)

func TestTextReporterMvToTmp(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.MvToTmp(
		runner.MvToTmpEvent{
			ActionFile:  "/repo/foo/.yamr.yaml",
			Source:      "/repo/foo/.generated",
			Destination: "/tmp/yamr/repo/foo/.generated",
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] mv_to_tmp: moving " +
		"/repo/foo/.generated to " +
		"/tmp/yamr/repo/foo/.generated\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterMkdir(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.Mkdir(
		runner.MkdirEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Path:       "/repo/foo/.generated",
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] mkdir: creating " +
		"/repo/foo/.generated\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterPreRunFailed(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.PreRunFailed(
		runner.PreRunFailedEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Operation:  runner.PreRunOperationMvToTmp,
			Err:        errors.New("destination already exists"),
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] mv_to_tmp failed: " +
		"destination already exists\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterActionStarted(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.ActionStarted(
		runner.ActionStartedEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Image:      "propero/yamr:candidate",
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] " +
		"propero/yamr:candidate triggered\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterActionFinishedSuccess(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.ActionFinished(
		runner.ActionFinishedEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Image:      "propero/yamr:candidate",
			Result: docker.Result{
				Ran:      true,
				ExitCode: 0,
			},
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] " +
		"propero/yamr:candidate succeeded\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterActionFinishedNonZero(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.ActionFinished(
		runner.ActionFinishedEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Image:      "propero/yamr:candidate",
			Result: docker.Result{
				Ran:      true,
				ExitCode: 42,
			},
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] " +
		"propero/yamr:candidate failed with status 42\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterActionFinishedExecutionFailure(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.ActionFinished(
		runner.ActionFinishedEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Image:      "propero/yamr:candidate",
			Err:        errors.New("execute docker: executable not found"),
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] " +
		"propero/yamr:candidate failed to execute Docker: " +
		"execute docker: executable not found\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterActionFinishedCancelled(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.ActionFinished(
		runner.ActionFinishedEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Image:      "propero/yamr:candidate",
			Err:        context.Canceled,
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] " +
		"propero/yamr:candidate cancelled: " +
		"context canceled\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterOnFailStarted(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.OnFailStarted(
		runner.OnFailStartedEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Image:      "propero/yamr-debug:candidate",
			StdoutPath: "/repo/foo/.yamr-debug/stdout.log",
			StderrPath: "/repo/foo/.yamr-debug/stderr.log",
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] on_fail: " +
		"propero/yamr-debug:candidate triggered " +
		"(stdout: /repo/foo/.yamr-debug/stdout.log, " +
		"stderr: /repo/foo/.yamr-debug/stderr.log)\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterOnFailFinished(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.OnFailFinished(
		runner.OnFailFinishedEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Image:      "propero/yamr-debug:candidate",
			Result: docker.Result{
				Ran:      true,
				ExitCode: 0,
			},
			StdoutPath: "/repo/foo/.yamr-debug/stdout.log",
			StderrPath: "/repo/foo/.yamr-debug/stderr.log",
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] on_fail: " +
		"propero/yamr-debug:candidate succeeded\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterNoActions(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.NoActions()

	want := "No triggered actions found.\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterSummary(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.Summary(
		runner.PlanResult{
			Actions: []runner.PlanActionResult{
				{
					Result: runner.ActionResult{},
				},
				{
					Result: runner.ActionResult{},
				},
				{
					Result: runner.ActionResult{
						FailedAt: runner.ActionStageRun,
					},
					Error: errors.New("action failed"),
				},
			},
		},
		nil,
	)

	want := "" +
		"Completed: 3 actions, 2 succeeded, 1 failed.\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterSummaryCancelled(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	result := runner.PlanResult{
		Actions: []runner.PlanActionResult{
			{
				Index: 0,
				State: runner.PlanActionRan,
				Result: runner.ActionResult{
					Action: runner.PlannedAction{
						Action: &action.Action{
							ActionFile: "/repo/a/.yamr.yaml",
						},
					},
				},
			},
			{
				Index: 1,
				State: runner.PlanActionRan,
				Result: runner.ActionResult{
					Action: runner.PlannedAction{
						Action: &action.Action{
							ActionFile: "/repo/b/.yamr.yaml",
						},
					},
				},
			},
			{
				Index: 2,
				State: runner.PlanActionRan,
				Result: runner.ActionResult{
					Action: runner.PlannedAction{
						Action: &action.Action{
							ActionFile: "/repo/c/.yamr.yaml",
						},
					},
				},
				Error: context.Canceled,
			},
			{
				Index: 3,
				State: runner.PlanActionNotRun,
				Result: runner.ActionResult{
					Action: runner.PlannedAction{
						Action: &action.Action{
							ActionFile: "/repo/d/.yamr.yaml",
						},
					},
				},
			},
			{
				Index: 4,
				State: runner.PlanActionNotRun,
				Result: runner.ActionResult{
					Action: runner.PlannedAction{
						Action: &action.Action{
							ActionFile: "/repo/e/.yamr.yaml",
						},
					},
				},
			},
		},
	}
	reporter.Summary(
		result,
		context.Canceled,
	)

	want := `Cancelled: 5 actions planned, 3 ran, 2 succeeded, 1 failed (1 cancelled), 2 not run.
Actions affected by cancellation:
  /repo/c/.yamr.yaml (cancelled while running)
  /repo/d/.yamr.yaml (not run)
  /repo/e/.yamr.yaml (not run)
`

	if got := output.String(); got != want {
		t.Fatalf(
			"output = %q, want %q",
			got,
			want,
		)
	}
}

func TestTextReporterDirtyGit(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.DirtyGit(
		git.Repository{
			Root: "/repo",
		},
		git.Status{
			Dirty: true,
			Lines: []string{
				" M internal/foo.go",
				"?? internal/bar.go",
			},
		},
	)

	want := "" +
		"NOTICE: Git repository /repo has uncommitted changes\n" +
		"   M internal/foo.go\n" +
		"  ?? internal/bar.go\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterIgnoredActionFiles(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.IgnoredActionFiles(
		[]discover.IgnoredActionFile{
			{
				ActionFile:  "/repo/foo/bar/.yamr.yaml",
				TriggeredBy: "/repo/foo/.yamr.yaml",
			},
			{
				ActionFile:  "/repo/foo/baz/.yamr.yaml",
				TriggeredBy: "/repo/foo/.yamr.yaml",
			},
		},
	)

	want := "" +
		"NOTICE: 2 action file(s) will not run because " +
		"an earlier action on their path was triggered:\n" +
		"  /repo/foo/bar/.yamr.yaml " +
		"(triggered by /repo/foo/.yamr.yaml)\n" +
		"  /repo/foo/baz/.yamr.yaml " +
		"(triggered by /repo/foo/.yamr.yaml)\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterOnFailPreparationFailed(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.OnFailPreparationFailed(
		runner.OnFailPreparationFailedEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Err: errors.New(
				"prepare stdout: permission denied",
			),
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] on_fail failed to prepare: " +
		"prepare stdout: permission denied\n"

	if got := output.String(); got != want {
		t.Fatalf(
			"output = %q, want %q",
			got,
			want,
		)
	}
}

func TestTextReporterOnFailFinishedNonZero(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.OnFailFinished(
		runner.OnFailFinishedEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Image:      "propero/yamr-debug:candidate",
			Result: docker.Result{
				Ran:      true,
				ExitCode: 7,
			},
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] on_fail: " +
		"propero/yamr-debug:candidate failed with status 7\n"

	if output.String() != want {
		t.Fatalf(
			"output = %q, want %q",
			output.String(),
			want,
		)
	}
}

func TestTextReporterOnFailFinishedExecutionFailure(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.OnFailFinished(
		runner.OnFailFinishedEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Image:      "jinal--shah/yamr:on-fail",
			Result:     docker.Result{},
			Err: errors.New(
				"exec: docker: executable file not found",
			),
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] on_fail: " +
		"jinal--shah/yamr:on-fail failed to execute Docker: " +
		"exec: docker: executable file not found\n"

	if got := output.String(); got != want {
		t.Fatalf(
			"output = %q, want %q",
			got,
			want,
		)
	}
}

func TestTextReporterOnFailFinishedCancelled(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.OnFailFinished(
		runner.OnFailFinishedEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Image:      "jinal--shah/yamr:on-fail",
			Result:     docker.Result{},
			Err: fmt.Errorf(
				"execute docker: %w",
				context.Canceled,
			),
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] on_fail: " +
		"jinal--shah/yamr:on-fail cancelled: execute docker: context canceled\n"

	if got := output.String(); got != want {
		t.Fatalf(
			"output = %q, want %q",
			got,
			want,
		)
	}
}

func TestTextReporterOnFailFinishedSuccess(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	reporter.OnFailFinished(
		runner.OnFailFinishedEvent{
			ActionFile: "/repo/foo/.yamr.yaml",
			Image:      "jinal--shah/yamr:on-fail",
			Result: docker.Result{
				Ran:      true,
				ExitCode: 0,
			},
		},
	)

	want := "" +
		"[/repo/foo/.yamr.yaml] on_fail: " +
		"jinal--shah/yamr:on-fail succeeded\n"

	if got := output.String(); got != want {
		t.Fatalf(
			"output = %q, want %q",
			got,
			want,
		)
	}
}

func TestTextReporterSummaryDistinguishesCancelledOnFail(
	t *testing.T,
) {
	var output bytes.Buffer

	reporter := NewTextReporter(
		&output,
	)

	result := runner.PlanResult{
		Actions: []runner.PlanActionResult{
			{
				Index: 0,
				State: runner.PlanActionRan,
				Result: runner.ActionResult{
					Action: runner.PlannedAction{
						Action: &action.Action{
							ActionFile: "/repo/a/.yamr.yaml",
						},
					},
					Docker: docker.Result{
						Ran:      true,
						ExitCode: 1,
					},
					OnFail: &runner.OnFailResult{
						Error: context.Canceled,
					},
				},
				Error: errors.Join(
					errors.New(
						"primary action failed",
					),
					context.Canceled,
				),
			},
			{
				Index: 1,
				State: runner.PlanActionNotRun,
				Result: runner.ActionResult{
					Action: runner.PlannedAction{
						Action: &action.Action{
							ActionFile: "/repo/b/.yamr.yaml",
						},
					},
				},
			},
		},
	}

	reporter.Summary(
		result,
		context.Canceled,
	)

	want := "" +
		"Cancelled: 2 actions planned, 1 ran, 0 succeeded, 1 failed (1 cancelled), 1 not run.\n" +
		"Actions affected by cancellation:\n" +
		"  /repo/a/.yamr.yaml (on_fail cancelled)\n" +
		"  /repo/b/.yamr.yaml (not run)\n"

	if got := output.String(); got != want {
		t.Fatalf(
			"output = %q, want %q",
			got,
			want,
		)
	}
}
