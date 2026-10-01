package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"jinal--shah/yamr-run/internal/action"
	"jinal--shah/yamr-run/internal/docker"
)

func TestRunAction(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a shell script")
	}

	installRunnerFakeDocker(
		t,
		`#!/bin/sh
printf 'docker stdout'
printf 'docker stderr' >&2
exit 0
`,
	)

	root := t.TempDir()

	output := filepath.Join(
		root,
		".generated",
	)

	if err := os.MkdirAll(
		output,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(output, "old.txt"),
		[]byte("old"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	planned := PlannedAction{
		Action: &action.Action{
			ActionFile: filepath.Join(
				root,
				".yamr.yaml",
			),
			ActionDir: root,
		},
		PreRun: []PreRunOperation{
			MoveToTmpOperation{
				Path: output,
			},
			MkdirOperation{
				Path: output,
				Mode: 0o750,
				UID:  os.Geteuid(),
				GID:  os.Getegid(),
			},
		},
		Docker: docker.Command{
			Image:      "example/image:latest",
			Entrypoint: "yamr",
			CmdSources: []string{
				"/sources/example.yaml",
			},
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	execution := NewExecution(nil)

	result, err := execution.RunAction(
		context.Background(),
		planned,
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf(
			"RunAction() error = %v",
			err,
		)
	}

	if result.Failed() {
		t.Fatalf(
			"result.Failed() = true, FailedAt = %q",
			result.FailedAt,
		)
	}

	if result.Action.Action != planned.Action {
		t.Fatal(
			"result does not contain planned action",
		)
	}

	if result.Docker.ExitCode != 0 {
		t.Fatalf(
			"Docker.ExitCode = %d, want 0",
			result.Docker.ExitCode,
		)
	}

	if stdout.String() != "docker stdout" {
		t.Fatalf(
			"stdout = %q",
			stdout.String(),
		)
	}

	if stderr.String() != "docker stderr" {
		t.Fatalf(
			"stderr = %q",
			stderr.String(),
		)
	}

	// mv_to_tmp followed by mkdir should leave a fresh output
	// directory behind.
	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf(
			"output directory missing: %v",
			err,
		)
	}

	if !info.IsDir() {
		t.Fatal(
			"output path is not a directory",
		)
	}

	if info.Mode().Perm() != 0o750 {
		t.Fatalf(
			"output mode = %04o, want 0750",
			info.Mode().Perm(),
		)
	}

	if _, err := os.Stat(
		filepath.Join(output, "old.txt"),
	); !os.IsNotExist(err) {
		t.Fatal(
			"old output still exists in recreated directory",
		)
	}

	tempRoot, err := execution.TempRoot()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tempRoot)
	})
}

func TestRunActionPreRunFailureStopsBeforeDocker(
	t *testing.T,
) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a shell script")
	}

	root := t.TempDir()

	dockerMarker := filepath.Join(
		root,
		"docker-ran",
	)

	installRunnerFakeDocker(
		t,
		`#!/bin/sh
touch "$DOCKER_MARKER"
exit 0
`,
	)

	t.Setenv(
		"DOCKER_MARKER",
		dockerMarker,
	)

	// Creating a file here means MkdirAll cannot create a child
	// directory beneath it.
	blocker := filepath.Join(
		root,
		"blocker",
	)

	if err := os.WriteFile(
		blocker,
		[]byte("not a directory"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(
		blocker,
		"output",
	)

	planned := PlannedAction{
		Action: &action.Action{
			ActionFile: filepath.Join(
				root,
				".yamr.yaml",
			),
		},
		PreRun: []PreRunOperation{
			MkdirOperation{
				Path: output,
				Mode: 0o755,
				UID:  os.Geteuid(),
				GID:  os.Getegid(),
			},
		},
		Docker: docker.Command{
			Image: "example/image:latest",
		},
	}

	execution := NewExecution(nil)

	result, err := execution.RunAction(
		context.Background(),
		planned,
		nil,
		nil,
	)

	if err == nil {
		t.Fatal(
			"RunAction() succeeded, want error",
		)
	}

	if !result.Failed() {
		t.Fatal(
			"result.Failed() = false, want true",
		)
	}

	if result.FailedAt != ActionStagePreRun {
		t.Fatalf(
			"FailedAt = %q, want %q",
			result.FailedAt,
			ActionStagePreRun,
		)
	}

	if !strings.Contains(
		err.Error(),
		"pre_run failed",
	) {
		t.Fatalf(
			"error = %q, want pre_run failure",
			err,
		)
	}

	if _, statErr := os.Stat(dockerMarker); !os.IsNotExist(statErr) {
		t.Fatalf(
			"Docker ran after pre_run failure; "+
				"stat error = %v",
			statErr,
		)
	}
}

func TestRunActionDockerFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a shell script")
	}

	installRunnerFakeDocker(
		t,
		`#!/bin/sh
printf 'yamr failed' >&2
exit 23
`,
	)

	root := t.TempDir()

	planned := PlannedAction{
		Action: &action.Action{
			ActionFile: filepath.Join(
				root,
				".yamr.yaml",
			),
		},
		Docker: docker.Command{
			Image: "example/image:latest",
		},
	}

	var stderr bytes.Buffer

	execution := NewExecution(nil)

	result, err := execution.RunAction(
		context.Background(),
		planned,
		nil,
		&stderr,
	)

	if err == nil {
		t.Fatal(
			"RunAction() succeeded, want error",
		)
	}

	if !result.Failed() {
		t.Fatal(
			"result.Failed() = false, want true",
		)
	}

	if result.FailedAt != ActionStageRun {
		t.Fatalf(
			"FailedAt = %q, want %q",
			result.FailedAt,
			ActionStageRun,
		)
	}

	if result.Docker.ExitCode != 23 {
		t.Fatalf(
			"Docker.ExitCode = %d, want 23",
			result.Docker.ExitCode,
		)
	}

	if stderr.String() != "yamr failed" {
		t.Fatalf(
			"stderr = %q, want yamr failed",
			stderr.String(),
		)
	}

	if !strings.Contains(
		err.Error(),
		"docker exited with status 23",
	) {
		t.Fatalf(
			"error = %q, want run failure",
			err,
		)
	}
}

func TestRunActionCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a shell script")
	}

	installRunnerFakeDocker(
		t,
		`#!/bin/sh
sleep 30
`,
	)

	root := t.TempDir()

	planned := PlannedAction{
		Action: &action.Action{
			ActionFile: filepath.Join(
				root,
				".yamr.yaml",
			),
		},
		Docker: docker.Command{
			Image: "example/image:latest",
		},
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		100*time.Millisecond,
	)
	defer cancel()

	execution := NewExecution(nil)

	result, err := execution.RunAction(
		ctx,
		planned,
		nil,
		nil,
	)

	if err == nil {
		t.Fatal(
			"RunAction() succeeded, want error",
		)
	}

	if result.FailedAt != ActionStageRun {
		t.Fatalf(
			"FailedAt = %q, want %q",
			result.FailedAt,
			ActionStageRun,
		)
	}

	if !strings.Contains(
		err.Error(),
		context.DeadlineExceeded.Error(),
	) {
		t.Fatalf(
			"error = %q, want deadline exceeded",
			err,
		)
	}
}

func TestRunActionRejectsNilContext(t *testing.T) {
	planned := PlannedAction{
		Action: &action.Action{
			ActionFile: "/repo/.yamr.yaml",
		},
	}

	execution := NewExecution(nil)

	result, err := execution.RunAction(
		nil,
		planned,
		nil,
		nil,
	)

	if err == nil {
		t.Fatal(
			"RunAction() succeeded, want error",
		)
	}

	if result.Failed() {
		t.Fatalf(
			"FailedAt = %q, want no execution stage",
			result.FailedAt,
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

func TestRunActionRejectsNilAction(t *testing.T) {
	execution := NewExecution(nil)

	result, err := execution.RunAction(
		context.Background(),
		PlannedAction{},
		nil,
		nil,
	)

	if err == nil {
		t.Fatal(
			"RunAction() succeeded, want error",
		)
	}

	if result.Failed() {
		t.Fatalf(
			"FailedAt = %q, want no execution stage",
			result.FailedAt,
		)
	}

	if !strings.Contains(
		err.Error(),
		"planned action has nil action",
	) {
		t.Fatalf(
			"error = %q",
			err,
		)
	}
}

func TestRunActionPreRunFailureDoesNotRunOnFail(
	t *testing.T,
) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a shell script")
	}

	root := t.TempDir()

	marker := filepath.Join(
		root,
		"on-fail-ran",
	)

	installRunnerFakeDocker(
		t,
		`#!/bin/sh
if [ "$1" = "on-fail" ]; then
    touch "$ON_FAIL_MARKER"
fi
exit 0
`,
	)

	t.Setenv(
		"ON_FAIL_MARKER",
		marker,
	)

	blocker := filepath.Join(
		root,
		"blocker",
	)

	if err := os.WriteFile(
		blocker,
		[]byte("file"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	planned := PlannedAction{
		Action: &action.Action{
			ActionFile: filepath.Join(
				root,
				".yamr.yaml",
			),
			ActionDir: root,
		},
		PreRun: []PreRunOperation{
			MkdirOperation{
				Path: filepath.Join(
					blocker,
					"output",
				),
				Mode: 0o755,
				UID:  os.Geteuid(),
				GID:  os.Getegid(),
			},
		},
		Docker: docker.Command{
			Image: "primary",
		},
		OnFail: &PlannedOnFail{
			Docker: docker.Command{
				Image: "on-fail",
				CmdOpts: []string{
					"on-fail",
				},
			},
		},
	}

	execution := NewExecution(nil)

	result, err := execution.RunAction(
		context.Background(),
		planned,
		nil,
		nil,
	)

	if err == nil {
		t.Fatal(
			"RunAction() succeeded, want error",
		)
	}

	if result.FailedAt != ActionStagePreRun {
		t.Fatalf(
			"FailedAt = %q, want %q",
			result.FailedAt,
			ActionStagePreRun,
		)
	}

	if result.OnFail != nil {
		t.Fatal(
			"OnFail != nil, want on_fail not executed",
		)
	}

	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal(
			"on_fail ran after pre_run failure",
		)
	}
}

func TestRunActionRunsOnFailAfterRunFailure(
	t *testing.T,
) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a shell script")
	}

	root := t.TempDir()

	installRunnerFakeDocker(
		t,
		`#!/bin/sh
case "$*" in
    *primary*)
        printf 'primary failed' >&2
        exit 17
        ;;
    *on-fail*)
        printf 'debug stdout'
        printf 'debug stderr' >&2
        exit 0
        ;;
esac

exit 99
`,
	)

	planned := PlannedAction{
		Action: &action.Action{
			ActionFile: filepath.Join(
				root,
				".yamr.yaml",
			),
			ActionDir: root,
		},
		Docker: docker.Command{
			Image: "primary",
			CmdOpts: []string{
				"primary",
			},
		},
		OnFail: &PlannedOnFail{
			Docker: docker.Command{
				Image: "debug",
				CmdOpts: []string{
					"on-fail",
				},
			},
			Stdout: OutputDestination{
				Path: filepath.Join(
					root,
					".yamr-debug",
					"stdout.log",
				),
			},
			Stderr: OutputDestination{
				Path: filepath.Join(
					root,
					".yamr-debug",
					"stderr.log",
				),
			},
		},
	}

	var stderr bytes.Buffer

	execution := NewExecution(nil)
	result, runErr := execution.RunAction(
		context.Background(),
		planned,
		nil,
		&stderr,
	)

	if runErr == nil {
		t.Fatal(
			"RunAction() succeeded, want primary error",
		)
	}

	if result.FailedAt != ActionStageRun {
		t.Fatalf(
			"FailedAt = %q, want %q",
			result.FailedAt,
			ActionStageRun,
		)
	}

	if result.Docker.ExitCode != 17 {
		t.Fatalf(
			"primary exit code = %d, want 17",
			result.Docker.ExitCode,
		)
	}

	if result.OnFail == nil {
		t.Fatal(
			"OnFail = nil, want result",
		)
	}

	if result.OnFail.Error != nil {
		t.Fatalf(
			"on_fail error = %v",
			result.OnFail.Error,
		)
	}

	if result.OnFail.Docker.ExitCode != 0 {
		t.Fatalf(
			"on_fail exit code = %d, want 0",
			result.OnFail.Docker.ExitCode,
		)
	}

	originalCommand, err := os.ReadFile(
		filepath.Join(
			root,
			".yamr-debug",
			"original.cmd",
		),
	)
	if err != nil {
		t.Fatalf(
			"read original.cmd: %v",
			err,
		)
	}

	onFailCommand, err := os.ReadFile(
		filepath.Join(
			root,
			".yamr-debug",
			"on_fail.cmd",
		),
	)
	if err != nil {
		t.Fatalf(
			"read on_fail.cmd: %v",
			err,
		)
	}

	wantOriginal := planned.Docker.ShellCommand() + "\n"

	if string(originalCommand) != wantOriginal {
		t.Fatalf(
			"original.cmd = %q, want %q",
			string(originalCommand),
			wantOriginal,
		)
	}

	wantOnFail := planned.OnFail.Docker.ShellCommand() + "\n"

	if string(onFailCommand) != wantOnFail {
		t.Fatalf(
			"on_fail.cmd = %q, want %q",
			string(onFailCommand),
			wantOnFail,
		)
	}

	stdoutContent, err := os.ReadFile(
		filepath.Join(
			root,
			".yamr-debug",
			"stdout.log",
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	if string(stdoutContent) != "debug stdout" {
		t.Fatalf(
			"debug stdout = %q",
			stdoutContent,
		)
	}

	stderrContent, err := os.ReadFile(
		filepath.Join(
			root,
			".yamr-debug",
			"stderr.log",
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	if string(stderrContent) != "debug stderr" {
		t.Fatalf(
			"debug stderr = %q",
			stderrContent,
		)
	}

	if !strings.Contains(
		runErr.Error(),
		"status 17",
	) {
		t.Fatalf(
			"error = %q, want primary status 17",
			runErr,
		)
	}
}

func TestRunActionEmitsActionEvents(
	t *testing.T,
) {
	events := &recordingEventSink{}
	execution := NewExecution(events)

	execution.runDocker = func(
		ctx context.Context,
		command docker.Command,
		stdout io.Writer,
		stderr io.Writer,
	) (docker.Result, error) {
		return docker.Result{
			ExitCode: 0,
			Ran:      true,
		}, nil
	}

	planned := PlannedAction{
		Action: &action.Action{
			ActionFile: SomeFile,
		},
		Docker: docker.Command{
			Image: "jinal--shah/yamr:test",
		},
	}

	_, err := execution.RunAction(
		context.Background(),
		planned,
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf(
			"RunAction() error = %v",
			err,
		)
	}

	if len(events.actionStarted) != 1 {
		t.Fatalf(
			"len(ActionStarted) = %d, want 1",
			len(events.actionStarted),
		)
	}

	started := events.actionStarted[0]

	if started.ActionFile != SomeFile {
		t.Fatalf(
			"ActionStarted.ActionFile = %q, want %q",
			started.ActionFile,
			SomeFile,
		)
	}

	if started.Image != planned.Docker.Image {
		t.Fatalf(
			"ActionStarted.Image = %q, want %q",
			started.Image,
			planned.Docker.Image,
		)
	}

	if len(events.actionFinished) != 1 {
		t.Fatalf(
			"len(ActionFinished) = %d, want 1",
			len(events.actionFinished),
		)
	}

	finished := events.actionFinished[0]

	if finished.ActionFile != SomeFile {
		t.Fatalf(
			"ActionFinished.ActionFile = %q, want %q",
			finished.ActionFile,
			SomeFile,
		)
	}

	if finished.Image != planned.Docker.Image {
		t.Fatalf(
			"ActionFinished.Image = %q, want %q",
			finished.Image,
			planned.Docker.Image,
		)
	}

	if finished.Result.ExitCode != 0 {
		t.Fatalf(
			"ActionFinished.Result.ExitCode = %d, want 0",
			finished.Result.ExitCode,
		)
	}

	if finished.Err != nil {
		t.Fatalf(
			"ActionFinished.Err = %v, want nil",
			finished.Err,
		)
	}

	if len(events.events) != 2 {
		t.Fatalf(
			"len(events) = %d, want 2",
			len(events.events),
		)
	}

	if events.events[0].Kind != "action_started" {
		t.Fatalf(
			"event 0 = %q, want action_started",
			events.events[0].Kind,
		)
	}

	if events.events[1].Kind != "action_finished" {
		t.Fatalf(
			"event 1 = %q, want action_finished",
			events.events[1].Kind,
		)
	}
}

func TestRunActionPreRunFailureDoesNotEmitActionEvents(
	t *testing.T,
) {
	root := t.TempDir()

	source := filepath.Join(
		root,
		"source",
	)

	if err := os.MkdirAll(
		source,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	events := &recordingEventSink{}
	execution := NewExecution(events)

	tempRoot, err := execution.TempRoot()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tempRoot)
	})

	destination, err := temporaryDestination(
		tempRoot,
		source,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(
		destination,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	dockerCalled := false

	execution.runDocker = func(
		ctx context.Context,
		command docker.Command,
		stdout io.Writer,
		stderr io.Writer,
	) (docker.Result, error) {
		dockerCalled = true
		return docker.Result{}, nil
	}

	planned := PlannedAction{
		Action: &action.Action{
			ActionFile: SomeFile,
		},
		PreRun: []PreRunOperation{
			MoveToTmpOperation{
				Path: source,
			},
		},
		Docker: docker.Command{
			Image: "jinal--shah/yamr:test",
		},
	}

	_, err = execution.RunAction(
		context.Background(),
		planned,
		io.Discard,
		io.Discard,
	)
	if err == nil {
		t.Fatal(
			"RunAction() error = nil, want error",
		)
	}

	if dockerCalled {
		t.Fatal(
			"Docker was called after pre_run failure",
		)
	}

	if len(events.actionStarted) != 0 {
		t.Fatalf(
			"len(ActionStarted) = %d, want 0",
			len(events.actionStarted),
		)
	}

	if len(events.actionFinished) != 0 {
		t.Fatalf(
			"len(ActionFinished) = %d, want 0",
			len(events.actionFinished),
		)
	}
}

func TestRunActionFailureEmitsActionFinishedEvent(
	t *testing.T,
) {
	events := &recordingEventSink{}
	execution := NewExecution(events)

	wantErr := errors.New(
		"docker failed",
	)

	execution.runDocker = func(
		ctx context.Context,
		command docker.Command,
		stdout io.Writer,
		stderr io.Writer,
	) (docker.Result, error) {
		return docker.Result{
			ExitCode: 42,
			Ran:      true,
		}, wantErr
	}

	planned := PlannedAction{
		Action: &action.Action{
			ActionFile: SomeFile,
		},
		Docker: docker.Command{
			Image: "jinal--shah/yamr:test",
		},
	}

	_, err := execution.RunAction(
		context.Background(),
		planned,
		io.Discard,
		io.Discard,
	)
	if err == nil {
		t.Fatal(
			"RunAction() error = nil, want error",
		)
	}

	if len(events.actionStarted) != 1 {
		t.Fatalf(
			"len(ActionStarted) = %d, want 1",
			len(events.actionStarted),
		)
	}

	if len(events.actionFinished) != 1 {
		t.Fatalf(
			"len(ActionFinished) = %d, want 1",
			len(events.actionFinished),
		)
	}

	finished := events.actionFinished[0]

	if finished.Result.ExitCode != 42 {
		t.Fatalf(
			"ExitCode = %d, want 42",
			finished.Result.ExitCode,
		)
	}

	if !errors.Is(
		finished.Err,
		wantErr,
	) {
		t.Fatalf(
			"Err = %v, want %v",
			finished.Err,
			wantErr,
		)
	}
}

func TestRunActionEmitsOnFailEvents(
	t *testing.T,
) {
	root := t.TempDir()

	events := &recordingEventSink{}
	execution := NewExecution(events)

	call := 0

	execution.runDocker = func(
		ctx context.Context,
		command docker.Command,
		stdout io.Writer,
		stderr io.Writer,
	) (docker.Result, error) {
		call++

		if call == 1 {
			return docker.Result{
					ExitCode: 1,
					Ran:      true,
				}, errors.New(
					"primary failed",
				)
		}

		return docker.Result{
			ExitCode: 0,
			Ran:      true,
		}, nil
	}

	stdoutPath := filepath.Join(
		root,
		"on-fail-stdout.log",
	)

	stderrPath := filepath.Join(
		root,
		"on-fail-stderr.log",
	)

	planned := PlannedAction{
		Action: &action.Action{
			ActionFile: SomeFile,
			ActionDir:  root,
		},
		Docker: docker.Command{
			Image: "jinal--shah/yamr:primary",
		},
		OnFail: &PlannedOnFail{
			Docker: docker.Command{
				Image: "jinal--shah/yamr:on-fail",
			},
			Stdout: OutputDestination{
				Path: stdoutPath,
			},
			Stderr: OutputDestination{
				Path: stderrPath,
			},
		},
	}

	_, err := execution.RunAction(
		context.Background(),
		planned,
		io.Discard,
		io.Discard,
	)
	if err == nil {
		t.Fatal(
			"RunAction() error = nil, want primary action error",
		)
	}

	if call != 2 {
		t.Fatalf(
			"docker calls = %d, want 2",
			call,
		)
	}

	if len(events.onFailStarted) != 1 {
		t.Fatalf(
			"len(OnFailStarted) = %d, want 1",
			len(events.onFailStarted),
		)
	}

	started := events.onFailStarted[0]

	if started.ActionFile != SomeFile {
		t.Fatalf(
			"OnFailStarted.ActionFile = %q, want %q",
			started.ActionFile,
			SomeFile,
		)
	}

	if started.Image != planned.OnFail.Docker.Image {
		t.Fatalf(
			"OnFailStarted.Image = %q, want %q",
			started.Image,
			planned.OnFail.Docker.Image,
		)
	}

	if started.StdoutPath != stdoutPath {
		t.Fatalf(
			"OnFailStarted.StdoutPath = %q, want %q",
			started.StdoutPath,
			stdoutPath,
		)
	}

	if started.StderrPath != stderrPath {
		t.Fatalf(
			"OnFailStarted.StderrPath = %q, want %q",
			started.StderrPath,
			stderrPath,
		)
	}

	if len(events.onFailFinished) != 1 {
		t.Fatalf(
			"len(OnFailFinished) = %d, want 1",
			len(events.onFailFinished),
		)
	}

	finished := events.onFailFinished[0]

	if finished.Result.ExitCode != 0 {
		t.Fatalf(
			"OnFailFinished.ExitCode = %d, want 0",
			finished.Result.ExitCode,
		)
	}

	if finished.Err != nil {
		t.Fatalf(
			"OnFailFinished.Err = %v, want nil",
			finished.Err,
		)
	}

	wantKinds := []string{
		"action_started",
		"action_finished",
		"on_fail_action_started",
		"on_fail_action_finished",
	}

	if len(events.events) != len(wantKinds) {
		t.Fatalf(
			"len(events) = %d, want %d",
			len(events.events),
			len(wantKinds),
		)
	}

	for i, want := range wantKinds {
		if events.events[i].Kind != want {
			t.Fatalf(
				"event %d = %q, want %q",
				i,
				events.events[i].Kind,
				want,
			)
		}
	}
}

func TestRunActionOnFailPreparationFailureDoesNotEmitOnFailStarted(
	t *testing.T,
) {
	root := t.TempDir()

	debugPath := filepath.Join(
		root,
		".yamr-debug",
	)

	if err := os.WriteFile(
		debugPath,
		[]byte("not a directory"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	events := &recordingEventSink{}
	execution := NewExecution(events)

	dockerCalls := 0

	execution.runDocker = func(
		ctx context.Context,
		command docker.Command,
		stdout io.Writer,
		stderr io.Writer,
	) (docker.Result, error) {
		dockerCalls++

		return docker.Result{
				ExitCode: 1,
				Ran:      true,
			}, errors.New(
				"primary failed",
			)
	}

	planned := PlannedAction{
		Action: &action.Action{
			ActionFile: SomeFile,
			ActionDir:  root,
		},
		Docker: docker.Command{
			Image: "jinal--shah/yamr:primary",
		},
		OnFail: &PlannedOnFail{
			Docker: docker.Command{
				Image: "jinal--shah/yamr:on-fail",
			},
		},
	}

	_, err := execution.RunAction(
		context.Background(),
		planned,
		io.Discard,
		io.Discard,
	)
	if err == nil {
		t.Fatal(
			"RunAction() error = nil, want error",
		)
	}

	// Only the primary Docker action should have run.
	if dockerCalls != 1 {
		t.Fatalf(
			"docker calls = %d, want 1",
			dockerCalls,
		)
	}

	if len(events.actionStarted) != 1 {
		t.Fatalf(
			"len(ActionStarted) = %d, want 1",
			len(events.actionStarted),
		)
	}

	if len(events.actionFinished) != 1 {
		t.Fatalf(
			"len(ActionFinished) = %d, want 1",
			len(events.actionFinished),
		)
	}

	if len(events.onFailStarted) != 0 {
		t.Fatalf(
			"len(OnFailStarted) = %d, want 0",
			len(events.onFailStarted),
		)
	}

	if len(events.onFailFinished) != 0 {
		t.Fatalf(
			"len(OnFailFinished) = %d, want 0",
			len(events.onFailFinished),
		)
	}

	if len(events.onFailPreparationFailed) != 1 {
		t.Fatalf(
			"len(OnFailPreparationFailed) = %d, want 1",
			len(events.onFailPreparationFailed),
		)
	}

	preparationFailed := events.onFailPreparationFailed[0]

	if preparationFailed.ActionFile != SomeFile {
		t.Errorf(
			"OnFailPreparationFailed.ActionFile = %q, want %q",
			preparationFailed.ActionFile,
			SomeFile,
		)
	}

	if preparationFailed.Err == nil {
		t.Fatal(
			"OnFailPreparationFailed.Err = nil, want error",
		)
	}

	if !strings.Contains(
		preparationFailed.Err.Error(),
		"create debug directory",
	) {
		t.Errorf(
			"OnFailPreparationFailed.Err = %q, want create debug directory error",
			preparationFailed.Err,
		)
	}

	wantEventKinds := []string{
		"action_started",
		"action_finished",
		"on_fail_preparation_failed",
	}

	if len(events.events) != len(wantEventKinds) {
		t.Fatalf(
			"len(events) = %d, want %d: %+v",
			len(events.events),
			len(wantEventKinds),
			events.events,
		)
	}

	for i, want := range wantEventKinds {
		if got := events.events[i].Kind; got != want {
			t.Errorf(
				"events[%d].Kind = %q, want %q",
				i,
				got,
				want,
			)
		}
	}
}

func TestRunActionCancellationDuringOnFailIsCancellation(
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

		if dockerCalls == 1 {
			return docker.Result{
				Ran:      true,
				ExitCode: 1,
			}, nil
		}

		close(onFailStarted)

		<-ctx.Done()

		return docker.Result{},
			ctx.Err()
	}

	planned := PlannedAction{
		Action: &action.Action{
			ActionFile: SomeFile,
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
	}

	type runResult struct {
		result ActionResult
		err    error
	}

	done := make(
		chan runResult,
		1,
	)

	go func() {
		result, err := execution.RunAction(
			ctx,
			planned,
			io.Discard,
			io.Discard,
		)

		done <- runResult{
			result: result,
			err:    err,
		}
	}()

	// protect test from hanging indefinitely
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
		if !errors.Is(
			got.err,
			context.Canceled,
		) {
			t.Fatalf(
				"RunAction() error = %v, want context.Canceled",
				got.err,
			)
		}

		if got.result.Docker.ExitCode != 1 {
			t.Errorf(
				"primary exit code = %d, want 1",
				got.result.Docker.ExitCode,
			)
		}

		if got.result.OnFail == nil {
			t.Fatal(
				"OnFail = nil, want result",
			)
		}

		if !errors.Is(
			got.result.OnFail.Error,
			context.Canceled,
		) {
			t.Errorf(
				"OnFail.Error = %v, want context.Canceled",
				got.result.OnFail.Error,
			)
		}
	case <-time.After(
		// protect test from hanging indefinitely
		time.Second,
	):
		t.Fatal(
			"timed out waiting for RunAction()",
		)
	}
}

func installRunnerFakeDocker(
	t *testing.T,
	script string,
) {
	t.Helper()

	dir := t.TempDir()

	path := filepath.Join(
		dir,
		"docker",
	)

	if err := os.WriteFile(
		path,
		[]byte(script),
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	t.Setenv(
		"PATH",
		dir+
			string(os.PathListSeparator)+
			os.Getenv("PATH"),
	)
}
