package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"jinal--shah/yamr-run/internal/apprunner"
	"jinal--shah/yamr-run/internal/runner"
	"jinal--shah/yamr-run/internal/cli"
)

func TestRunCLIParseFailure(
	t *testing.T,
) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	runApp := func(
		ctx context.Context,
		options apprunner.Options,
	) (apprunner.Result, error) {
		t.Fatal(
			"app runner called after CLI parse failure",
		)

		return apprunner.Result{}, nil
	}

	exitCode := run(
		context.Background(),
		nil,
		nil,
		bytes.NewBuffer(nil),
		&stdout,
		&stderr,
		failedCLIParser(),
		dummyWd,
		runApp,
	)

	if exitCode != exitFailure {
		t.Errorf(
			"exit code = %d, want %d",
			exitCode,
			exitFailure,
		)
	}

	if stdout.Len() != 0 {
		t.Errorf(
			"stdout = %q, want empty",
			stdout.String(),
		)
	}

	wantStderr := "yamr-runner: bad CLI options\n"

	if got := stderr.String(); got != wantStderr {
		t.Errorf(
			"stderr = %q, want %q",
			got,
			wantStderr,
		)
	}
}

func TestRunSuccess(
	t *testing.T,
) {
	runApp := func(
		ctx context.Context,
		options apprunner.Options,
	) (apprunner.Result, error) {
		return apprunner.Result{
			Executed: true,
		}, nil
	}

	exitCode := run(
		context.Background(),
		nil,
		nil,
		bytes.NewBuffer(nil),
		io.Discard,
		io.Discard,
		successfulCLIParser(),
		dummyWd,
		runApp,
	)

	if exitCode != exitSuccess {
		t.Errorf(
			"exit code = %d, want %d",
			exitCode,
			exitSuccess,
		)
	}
}

func TestRunDeclinedIsSuccess(
	t *testing.T,
) {
	runApp := func(
		ctx context.Context,
		options apprunner.Options,
	) (apprunner.Result, error) {
		return apprunner.Result{
			Declined: true,
		}, nil
	}

	exitCode := run(
		context.Background(),
		nil,
		nil,
		bytes.NewBuffer(nil),
		io.Discard,
		io.Discard,
		successfulCLIParser(),
		dummyWd,
		runApp,
	)

	if exitCode != exitSuccess {
		t.Errorf(
			"exit code = %d, want %d",
			exitCode,
			exitSuccess,
		)
	}
}

func TestRunActionFailure(
	t *testing.T,
) {
	runApp := func(
		ctx context.Context,
		options apprunner.Options,
	) (apprunner.Result, error) {
		return apprunner.Result{
			Executed: true,
			Execution: runner.PlanResult{
				Actions: []runner.PlanActionResult{
					{
						State: runner.PlanActionRan,
						Error: errors.New(
							"action failed",
						),
					},
				},
			},
		}, nil
	}

	exitCode := run(
		context.Background(),
		nil,
		nil,
		bytes.NewBuffer(nil),
		io.Discard,
		io.Discard,
		successfulCLIParser(),
		dummyWd,
		runApp,
	)

	if exitCode != exitFailure {
		t.Errorf(
			"exit code = %d, want %d",
			exitCode,
			exitFailure,
		)
	}
}

func TestRunApplicationError(
	t *testing.T,
) {
	var stderr bytes.Buffer

	runApp := func(
		ctx context.Context,
		options apprunner.Options,
	) (apprunner.Result, error) {
		return apprunner.Result{},
			context.Canceled
	}

	exitCode := run(
		context.Background(),
		nil,
		nil,
		bytes.NewBuffer(nil),
		io.Discard,
		&stderr,
		successfulCLIParser(),
		dummyWd,
		runApp,
	)

	if exitCode != exitFailure {
		t.Errorf(
			"exit code = %d, want %d",
			exitCode,
			exitFailure,
		)
	}

	if !strings.Contains(
		stderr.String(),
		context.Canceled.Error(),
	) {
		t.Errorf(
			"stderr = %q, want cancellation error",
			stderr.String(),
		)
	}
}

// check cancelled context passed to injected apprunner
// i.e. main.go run() context is same context in apprunner.go Run()
// That's important for signal handling to propagate e.g so CTRL-C gets to docker.
func TestRunPassesContextToApplication(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	runApp := func(
		gotCtx context.Context,
		options apprunner.Options,
	) (apprunner.Result, error) {
		if !errors.Is(
			gotCtx.Err(),
			context.Canceled,
		) {
			t.Errorf(
				"context error = %v, want context.Canceled",
				gotCtx.Err(),
			)
		}

		return apprunner.Result{},
			gotCtx.Err()
	}

	exitCode := run(
		ctx,
		nil,
		nil,
		bytes.NewBuffer(nil),
		io.Discard,
		io.Discard,
		successfulCLIParser(),
		dummyWd,
		runApp,
	)

	if exitCode != exitFailure {
		t.Errorf(
			"exit code = %d, want %d",
			exitCode,
			exitFailure,
		)
	}
}

func TestRunPassesWorkingDirectoryToAppRunner(
	t *testing.T,
) {
	const wantRunDir = "/repo/work/tree"

	getwd := func() (string, error) {
		return wantRunDir, nil
	}

	var gotOptions apprunner.Options

	runApp := func(
		ctx context.Context,
		options apprunner.Options,
	) (apprunner.Result, error) {
		gotOptions = options

		return apprunner.Result{}, nil
	}

	exitCode := run(
		context.Background(),
		nil,
		nil,
		strings.NewReader(""),
		io.Discard,
		io.Discard,
		successfulCLIParser(),
		getwd,
		runApp,
	)

	if exitCode != exitSuccess {
		t.Fatalf(
			"exit code = %d, want %d",
			exitCode,
			exitSuccess,
		)
	}

	if gotOptions.RunDir != wantRunDir {
		t.Fatalf(
			"RunDir = %q, want %q",
			gotOptions.RunDir,
			wantRunDir,
		)
	}
}

func TestRunWorkingDirectoryErrorDoesNotRunApp(
	t *testing.T,
) {
	getwdErr := errors.New(
		"getwd failed",
	)

	getwd := func() (string, error) {
		return "", getwdErr
	}

	appCalled := false

	runApp := func(
		ctx context.Context,
		options apprunner.Options,
	) (apprunner.Result, error) {
		appCalled = true

		return apprunner.Result{}, nil
	}

	var stderr bytes.Buffer

	exitCode := run(
		context.Background(),
		nil,
		nil,
		strings.NewReader(""),
		io.Discard,
		&stderr,
		successfulCLIParser(),
		getwd,
		runApp,
	)

	if exitCode != exitFailure {
		t.Fatalf(
			"exit code = %d, want %d",
			exitCode,
			exitFailure,
		)
	}

	if appCalled {
		t.Fatal(
			"app runner called, want no call",
		)
	}

	wantStderr := "yamr-runner: could not get get current working directory: getwd failed\n"

	if stderr.String() != wantStderr {
		t.Fatalf(
			"stderr = %q, want %q",
			stderr.String(),
			wantStderr,
		)
	}
}

func successfulCLIParser() cliParserFunc {
	return func(
		args []string,
		environ []string,
	) (cli.Options, error) {
		return cli.Options{
			ConfigFile:       "/config.yaml",
			YamrSourcesDir:   "/sources",
			YamrSourceLabels: "/labels.yaml",
		}, nil
	}
}

func failedCLIParser() cliParserFunc {
	return func(
		args []string,
		environ []string,
	) (cli.Options, error) {
		return cli.Options{},
			errors.New("bad CLI options")
	}
}

func dummyWd() (string, error) {
	return "/I/am/here", nil
}
