package report

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"jinal--shah/yamr-runner/internal/discover"
	"jinal--shah/yamr-runner/internal/git"
	"jinal--shah/yamr-runner/internal/runner"
)

type Reporter interface {
	runner.EventSink

	DirtyGit(
		repository git.Repository,
		status git.Status,
	)

	IgnoredActionFiles(
		files []discover.IgnoredActionFile,
	)

	NoActions()

	PullingDockerImages(
		images []string,
	)

	DockerImagesReady()

	Summary(
		result runner.PlanResult,
		err error,
	)
}

type TextReporter struct {
	mu sync.Mutex
	w  io.Writer
}

func NewTextReporter(
	w io.Writer,
) *TextReporter {
	return &TextReporter{
		w: w,
	}
}

var _ Reporter = (*TextReporter)(nil)

func (r *TextReporter) DirtyGit(
	repository git.Repository,
	status git.Status,
) {
	r.writef(
		"NOTICE: Git repository %s has uncommitted changes\n",
		repository.Root,
	)

	for _, line := range status.Lines {
		r.writef(
			"  %s\n",
			line,
		)
	}
}

func (r *TextReporter) IgnoredActionFiles(
	files []discover.IgnoredActionFile,
) {
	if len(files) == 0 {
		return
	}

	r.writef(
		"NOTICE: %d action file(s) will not run because an earlier action on their path was triggered:\n",
		len(files),
	)

	for _, file := range files {
		r.writef(
			"  %s (triggered by %s)\n",
			file.ActionFile,
			file.TriggeredBy,
		)
	}
}

func (r *TextReporter) NoActions() {
	r.writef(
		"No triggered actions found.\n",
	)
}

func (r *TextReporter) PullingDockerImages(
	images []string,
) {
	num_label := "images"
	if len(images) == 1 {
		num_label = "image"
	}
	r.writef(
		"Pulling %d Docker %s...\n",
		len(images),
		num_label,
	)

	for _, image := range images {
		r.writef(
			"  %s\n",
			image,
		)
	}
}

func (r *TextReporter) DockerImagesReady() {
	r.writef(
		"Docker images ready.\n",
	)
}

func (r *TextReporter) MvToTmp(
	event runner.MvToTmpEvent,
) {
	r.writef(
		"[%s] mv_to_tmp: moving %s to %s\n",
		event.ActionFile,
		event.Source,
		event.Destination,
	)
}

func (r *TextReporter) Mkdir(
	event runner.MkdirEvent,
) {
	r.writef(
		"[%s] mkdir: creating %s\n",
		event.ActionFile,
		event.Path,
	)
}

func (r *TextReporter) PreRunFailed(
	event runner.PreRunFailedEvent,
) {
	r.writef(
		"[%s] %s failed: %v\n",
		event.ActionFile,
		event.Operation,
		event.Err,
	)
}

func (r *TextReporter) ActionStarted(
	event runner.ActionStartedEvent,
) {
	r.writef(
		"[%s] %s triggered\n",
		event.ActionFile,
		event.Image,
	)
}

func (r *TextReporter) ActionFinished(
	event runner.ActionFinishedEvent,
) {
	switch {
	case event.Err != nil:
		if errors.Is(
			event.Err,
			context.Canceled,
		) {
			r.writef(
				"[%s] %s cancelled: %v\n",
				event.ActionFile,
				event.Image,
				event.Err,
			)
			return
		}

		r.writef(
			"[%s] %s failed to execute Docker: %v\n",
			event.ActionFile,
			event.Image,
			event.Err,
		)

	case !event.Result.Ran:
		// Defensive handling for a result that cannot normally be
		// produced by docker.Run().
		r.writef(
			"[%s] %s failed: Docker did not run\n",
			event.ActionFile,
			event.Image,
		)

	case event.Result.ExitCode != 0:
		r.writef(
			"[%s] %s failed with status %d\n",
			event.ActionFile,
			event.Image,
			event.Result.ExitCode,
		)

	default:
		r.writef(
			"[%s] %s succeeded\n",
			event.ActionFile,
			event.Image,
		)
	}
}

func (r *TextReporter) OnFailPreparationFailed(
	event runner.OnFailPreparationFailedEvent,
) {
	r.writef(
		"[%s] on_fail failed to prepare: %v\n",
		event.ActionFile,
		event.Err,
	)
}

func (r *TextReporter) OnFailStarted(
	event runner.OnFailStartedEvent,
) {
	r.writef(
		"[%s] on_fail: %s triggered%s\n",
		event.ActionFile,
		event.Image,
		formatOutputPaths(
			event.StdoutPath,
			event.StderrPath,
		),
	)
}

func (r *TextReporter) OnFailFinished(
	event runner.OnFailFinishedEvent,
) {
	switch {
	case event.Err != nil:
		if errors.Is(
			event.Err,
			context.Canceled,
		) {
			r.writef(
				"[%s] on_fail: %s cancelled: %v\n",
				event.ActionFile,
				event.Image,
				event.Err,
			)
			return
		}

		r.writef(
			"[%s] on_fail: %s failed to execute Docker: %v\n",
			event.ActionFile,
			event.Image,
			event.Err,
		)

	case !event.Result.Ran:
		r.writef(
			"[%s] on_fail: %s failed: Docker did not run\n",
			event.ActionFile,
			event.Image,
		)

	case event.Result.ExitCode != 0:
		r.writef(
			"[%s] on_fail: %s failed with status %d\n",
			event.ActionFile,
			event.Image,
			event.Result.ExitCode,
		)

	default:
		r.writef(
			"[%s] on_fail: %s succeeded\n",
			event.ActionFile,
			event.Image,
		)
	}
}

func (r *TextReporter) Summary(
	result runner.PlanResult,
	err error,
) {
	summary := result.Summary()

	if errors.Is(
		err,
		context.Canceled,
	) {
		r.writef(
			"Cancelled: %d actions planned, %d ran, %d succeeded, %d failed (%d cancelled), %d not run.\n",
			summary.Total,
			summary.Ran,
			summary.Succeeded,
			summary.Failed,
			summary.Cancelled,
			summary.NotRun,
		)

		if summary.Cancelled > 0 ||
			summary.NotRun > 0 {
			r.writef(
				"Actions affected by cancellation:\n",
			)

			for _, action := range result.Actions {
				var reason string

				switch {
				case action.State == runner.PlanActionNotRun:
					reason = "not run"

				case action.Result.OnFail != nil &&
					errors.Is(
						action.Result.OnFail.Error,
						context.Canceled,
					):
					reason = "on_fail cancelled"

				case errors.Is(
					action.Error,
					context.Canceled,
				):
					reason = "cancelled while running"

				default:
					continue
				}

				actionFile := action.
					Result.
					Action.
					Action.
					ActionFile

				r.writef(
					"  %s (%s)\n",
					actionFile,
					reason,
				)
			}

		}

		return
	}

	actionsLabel := "actions"
	if summary.Total == 1 {
		actionsLabel = "action"
	}

	r.writef(
		"Completed: %d %s, %d succeeded, %d failed.\n",
		summary.Total,
		actionsLabel,
		summary.Succeeded,
		summary.Failed,
	)
}

func (r *TextReporter) writef(
	format string,
	args ...any,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, _ = fmt.Fprintf(
		r.w,
		format,
		args...,
	)
}

func formatOutputPaths(
	stdoutPath string,
	stderrPath string,
) string {
	switch {
	case stdoutPath != "" &&
		stderrPath != "":
		return fmt.Sprintf(
			" (stdout: %s, stderr: %s)",
			stdoutPath,
			stderrPath,
		)

	case stdoutPath != "":
		return fmt.Sprintf(
			" (stdout: %s)",
			stdoutPath,
		)

	case stderrPath != "":
		return fmt.Sprintf(
			" (stderr: %s)",
			stderrPath,
		)

	default:
		return ""
	}
}
