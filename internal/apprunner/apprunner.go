package apprunner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"jinal--shah/yamr-run/internal/app"
	"jinal--shah/yamr-run/internal/cli"
	"jinal--shah/yamr-run/internal/report"
	"jinal--shah/yamr-run/internal/runner"
)

type Options struct {
	CLI     cli.Options
	RunDir  string
	HostEnv []string

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type Result struct {
	Build     app.BuildResult
	Execution runner.PlanResult
	Executed  bool
	Declined  bool
}

// buildFunc allows testing of app orchestration
// without invoking the real build pipeline.
type buildFunc func(
	context.Context,
	app.BuildOptions,
) (app.BuildResult, error)

// executeFunc allows testing of app orchestration
// without executing real docker
type executeFunc func(
	context.Context,
	runner.Plan,
	int,
	io.Writer,
	io.Writer,
) (runner.PlanResult, error)

// applicationRunner allows testing of app orchestration
// without needing to call the public Run() which would
// need external deps like docker and git to actually run
type applicationRunner struct {
	build   buildFunc
	execute executeFunc
}

func Run(
	ctx context.Context,
	options Options,
) (Result, error) {
	if options.Stderr == nil {
		return Result{}, fmt.Errorf(
			"stderr must not be nil",
		)
	}

	reporter := report.NewTextReporter(
		options.Stderr,
	)

	execution := runner.NewExecution(reporter)

	r := applicationRunner{
		build: app.Build,
		execute: execution.RunPlan,
	}

	return r.run(
		ctx,
		options,
		reporter,
	)
}

func (r applicationRunner) run(
	ctx context.Context,
	options Options,
	reporter report.Reporter,
) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf(
			"context must not be nil",
		)
	}

	if options.Stdin == nil {
		return Result{}, fmt.Errorf(
			"stdin must not be nil",
		)
	}

	if options.Stdout == nil {
		return Result{}, fmt.Errorf(
			"stdout must not be nil",
		)
	}

	if options.Stderr == nil {
		return Result{}, fmt.Errorf(
			"stderr must not be nil",
		)
	}

	if reporter == nil {
		return Result{}, fmt.Errorf(
			"reporter must not be nil",
		)
	}

	build, err := r.build(
		ctx,
		app.BuildOptions{
			CLI:     options.CLI,
			RunDir:  options.RunDir,
			HostEnv: options.HostEnv,
		},
	)
	if err != nil {
		return Result{}, fmt.Errorf(
			"build application: %w",
			err,
		)
	}

	wouldPrompt := false

	if build.GitStatus.Dirty {
		wouldPrompt = true
		reporter.DirtyGit(
			build.Repository,
			build.GitStatus,
		)
	}

	if len(build.IgnoredActionFiles) > 0 {
		wouldPrompt = true
		reporter.IgnoredActionFiles(
			build.IgnoredActionFiles,
		)
	}

	if len(build.Plan.Actions) == 0 {
		reporter.NoActions()

		return Result{
			Build: build,
		}, nil
	}

	if wouldPrompt && !options.CLI.NoPrompts {
		confirmed, err := confirmContinue(
			options.Stdin,
			options.Stderr,
		)
		if err != nil {
			return Result{
				Build: build,
			}, err
		}

		if !confirmed {
			return Result{
				Build:    build,
				Declined: true,
			}, nil
		}
	}

	planResult, err := r.execute(
		ctx,
		build.Plan,
		options.CLI.MaxWorkers,
		options.Stdout,
		options.Stderr,
	)

	result := Result{
		Build:     build,
		Execution: planResult,
		Executed:  true,
	}

	reporter.Summary(
		planResult,
		err,
	)
	if err != nil {
		return result , fmt.Errorf(
			"execute plan: %w",
			err,
		)
	}

	return result, nil
}

func confirmContinue(
	reader io.Reader,
	writer io.Writer,
) (bool, error) {
	fmt.Fprint(
		writer,
		"Continue? [y/N] ",
	)

	scanner := bufio.NewScanner(
		reader,
	)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf(
				"reading confirmation: %w",
				err,
			)
		}

		//
		// EOF without an answer is treated as declining.
		//
		return false, nil
	}

	answer := strings.ToLower(
		strings.TrimSpace(
			scanner.Text(),
		),
	)

	switch answer {
	case "y", "yes":
		return true, nil

	default:
		return false, nil
	}
}
