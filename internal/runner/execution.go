package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"jinal--shah/yamr-runner/internal/docker"
)

type Execution struct {
	tempOnce sync.Once
	tempRoot string
	tempErr  error

	runDocker dockerRunnerFunc
	events    EventSink
}

func NewExecution(
	events EventSink,
) *Execution {
	return &Execution{
		events: events,
	}
}

type dockerRunnerFunc func(
	context.Context,
	docker.Command,
	io.Writer,
	io.Writer,
) (docker.Result, error)

func (e *Execution) dockerRun(
	ctx context.Context,
	command docker.Command,
	stdout io.Writer,
	stderr io.Writer,
) (docker.Result, error) {
	if e.runDocker != nil {
		return e.runDocker(
			ctx,
			command,
			stdout,
			stderr,
		)
	}

	return docker.Run(
		ctx,
		command,
		stdout,
		stderr,
	)
}

type ActionStage string

const (
	ActionStagePreRun ActionStage = "pre_run"
	ActionStageRun    ActionStage = "run"
)

type ActionResult struct {
	Action   PlannedAction
	Docker   docker.Result
	OnFail   *OnFailResult
	FailedAt ActionStage
}

type OnFailResult struct {
	Docker docker.Result
	Error  error
}

func (r ActionResult) Failed() bool {
	return r.FailedAt != ""
}

// RunAction executes one complete planned action.
//
// It runs pre_run operations followed by the primary Docker action.
// If the primary action fails and an on_fail action is configured,
// the on_fail action is executed before RunAction returns.
func (e *Execution) RunAction(
	ctx context.Context,
	planned PlannedAction,
	stdout io.Writer,
	stderr io.Writer,
) (ActionResult, error) {
	result := ActionResult{
		Action: planned,
	}

	if ctx == nil {
		return result, fmt.Errorf(
			"context must not be nil",
		)
	}

	if planned.Action == nil {
		return result, fmt.Errorf(
			"planned action has nil action",
		)
	}

	if err := e.RunPreRun(
		planned.Action.ActionFile,
		planned.PreRun,
	); err != nil {
		result.FailedAt = ActionStagePreRun
		return result, fmt.Errorf(
			"action %q pre_run failed: %w",
			planned.Action.ActionFile,
			err,
		)
	}

	e.emitActionStarted(
		ActionStartedEvent{
			ActionFile: planned.Action.ActionFile,
			Image:      planned.Docker.Image,
		},
	)

	dockerResult, err := e.dockerRun(
		ctx,
		planned.Docker,
		stdout,
		stderr,
	)

	e.emitActionFinished(
		ActionFinishedEvent{
			ActionFile: planned.Action.ActionFile,
			Image:      planned.Docker.Image,
			Result:     dockerResult,
			Err:        err,
		},
	)

	result.Docker = dockerResult

	if err != nil || !dockerResult.Successful() {
		result.FailedAt = ActionStageRun

		var primaryErr error

		switch {
		case err != nil:
			primaryErr = fmt.Errorf(
				"action %q could not run docker: %w",
				planned.Action.ActionFile,
				err,
			)

		case !dockerResult.Ran:
			primaryErr = fmt.Errorf(
				"action %q docker did not run",
				planned.Action.ActionFile,
			)

		default:
			primaryErr = fmt.Errorf(
				"action %q docker exited with status %d",
				planned.Action.ActionFile,
				dockerResult.ExitCode,
			)
		}

		if planned.OnFail == nil {
			return result, primaryErr
		}

		onFailResult := e.runOnFail(
			ctx,
			planned,
			stdout,
			stderr,
		)

		result.OnFail = &onFailResult

		if onFailResult.Error != nil {
			return result, errors.Join(
				primaryErr,
				fmt.Errorf(
					"on_fail also failed: %w",
					onFailResult.Error,
				),
			)
		}

		return result, primaryErr
	}

	return result, nil

}

func (e *Execution) returnOnFailPreparationFailed(
	actionFile string,
	err error,
) OnFailResult {
	e.emitOnFailPreparationFailed(
		OnFailPreparationFailedEvent{
			ActionFile: actionFile,
			Err:        err,
		},
	)

	return OnFailResult{
		Error: err,
	}
}

func (e *Execution) runOnFail(
	ctx context.Context,
	planned PlannedAction,
	inheritedStdout io.Writer,
	inheritedStderr io.Writer,
) OnFailResult {
	onFail := planned.OnFail

	if onFail == nil {
		return OnFailResult{}
	}

	debugDir := filepath.Join(
		planned.Action.ActionDir,
		".yamr-debug",
	)

	if err := os.MkdirAll(
		debugDir,
		0o755,
	); err != nil {
		return e.returnOnFailPreparationFailed(
			planned.Action.ActionFile,
			fmt.Errorf(
				"create debug directory %q: %w",
				debugDir,
				err,
			),
		)
	}

	originalCmd := filepath.Join(
		debugDir,
		"original.cmd",
	)
	if err := writeDebugCommand(
		originalCmd,
		planned.Docker,
	); err != nil {
		return e.returnOnFailPreparationFailed(
			planned.Action.ActionFile,
			fmt.Errorf(
				"create original.cmd %q: %w",
				originalCmd,
				err,
			),
		)
	}

	onFailCmd := filepath.Join(
		debugDir,
		"on_fail.cmd",
	)
	if err := writeDebugCommand(
		onFailCmd,
		planned.OnFail.Docker,
	); err != nil {
		return e.returnOnFailPreparationFailed(
			planned.Action.ActionFile,
			fmt.Errorf(
				"create on_fail.cmd %q: %w",
				onFailCmd,
				err,
			),
		)
	}

	stdout, closeStdout, err := openOnFailOutput(
		onFail.Stdout,
		inheritedStdout,
	)
	if err != nil {
		return e.returnOnFailPreparationFailed(
			planned.Action.ActionFile,
			fmt.Errorf(
				"prepare stdout: %w",
				err,
			),
		)
	}

	defer closeStdout()

	stderr, closeStderr, err := openOnFailOutput(
		onFail.Stderr,
		inheritedStderr,
	)
	if err != nil {
		return e.returnOnFailPreparationFailed(
			planned.Action.ActionFile,
			fmt.Errorf(
				"prepare stderr: %w",
				err,
			),
		)
	}

	defer closeStderr()

	e.emitOnFailStarted(
		OnFailStartedEvent{
			ActionFile: planned.Action.ActionFile,
			Image:      onFail.Docker.Image,
			StdoutPath: onFailOutputPath(onFail.Stdout),
			StderrPath: onFailOutputPath(onFail.Stderr),
		},
	)

	dockerResult, err := e.dockerRun(
		ctx,
		onFail.Docker,
		stdout,
		stderr,
	)

	e.emitOnFailFinished(
		OnFailFinishedEvent{
			ActionFile: planned.Action.ActionFile,
			Image:      onFail.Docker.Image,
			Result:     dockerResult,
			Err:        err,
			StdoutPath: onFailOutputPath(onFail.Stdout),
			StderrPath: onFailOutputPath(onFail.Stderr),
		},
	)

	return OnFailResult{
		Docker: dockerResult,
		Error:  err,
	}
}

func openOnFailOutput(
	destination OutputDestination,
	inherited io.Writer,
) (
	io.Writer,
	func() error,
	error,
) {
	if destination.Inherit {
		return inherited, func() error {
			return nil
		}, nil
	}

	if destination.Path == "" {
		return nil, nil, fmt.Errorf(
			"output path is empty",
		)
	}

	file, err := os.OpenFile(
		destination.Path,
		os.O_WRONLY|
			os.O_CREATE|
			os.O_TRUNC,
		0o644,
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"open %q: %w",
			destination.Path,
			err,
		)
	}

	return file, file.Close, nil
}

func onFailOutputPath(
	destination OutputDestination,
) string {
	if destination.Inherit {
		return ""
	}

	return destination.Path
}

func writeDebugCommand(
	path string,
	command docker.Command,
) error {
	if err := os.WriteFile(
		path,
		[]byte(command.ShellCommand()+"\n"),
		0644,
	); err != nil {
		return fmt.Errorf(
			"write Docker command %q: %w",
			path,
			err,
		)
	}

	return nil
}
