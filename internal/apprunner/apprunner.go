package apprunner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"jinal--shah/yamr-runner/internal/app"
	"jinal--shah/yamr-runner/internal/cli"
	"jinal--shah/yamr-runner/internal/docker"
	"jinal--shah/yamr-runner/internal/report"
	"jinal--shah/yamr-runner/internal/runner"
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

// prepareFunc allows testing of app orchestration
// without requiring Docker to be available.
type prepareFunc func(
	context.Context,
	runner.Plan,
	int,
) error

// pullImageFunc allows testing of app orchestration
// without requiring Docker to be available.
type pullImageFunc func(
	context.Context,
	string,
) error

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
	prepare prepareFunc
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
		build:   app.Build,
		prepare: prepare,
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

	images := build.Plan.Images()

	if len(images) > 0 {
		reporter.PullingDockerImages(
			images,
		)
	}

	if len(images) > 0 {
		reporter.DockerImagesReady()
	}

	err = r.prepare(
		ctx,
		build.Plan,
		options.CLI.MaxWorkers,
	)
	if err != nil {
		return Result{
				Build: build,
			}, fmt.Errorf(
				"prepare execution: %w",
				err,
			)
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
		return result, fmt.Errorf(
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

// checking docker exists, pulling images
func prepare(
	ctx context.Context,
	plan runner.Plan,
	maxWorkers int,
) error {
	return prepareExecution(
		ctx,
		plan,
		maxWorkers,
		docker.Check,
		docker.Pull,
	)
}

func prepareExecution(
	ctx context.Context,
	plan runner.Plan,
	maxWorkers int,
	checkDocker func(context.Context) error,
	pullImage pullImageFunc,
) error {
	if ctx == nil {
		return fmt.Errorf(
			"context must not be nil",
		)
	}

	if maxWorkers <= 0 {
		return fmt.Errorf(
			"max workers must be greater than zero",
		)
	}

	if err := checkDocker(
		ctx,
	); err != nil {
		return err
	}

	images := plan.Images()

	if len(images) == 0 {
		return nil
	}

	workerCount := min(
		maxWorkers,
		len(images),
	)

	type pullResult struct {
		image string
		err   error
	}

	jobs := make(
		chan string,
	)

	results := make(
		chan pullResult,
		len(images),
	)

	var workers sync.WaitGroup

	for range workerCount {
		workers.Add(1)

		go func() {
			defer workers.Done()

			for image := range jobs {
				err := pullImage(
					ctx,
					image,
				)

				results <- pullResult{
					image: image,
					err:   err,
				}
			}
		}()
	}

	go func() {
		defer close(
			jobs,
		)

		for _, image := range images {
			select {
			case jobs <- image:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		workers.Wait()

		close(
			results,
		)
	}()

	var pullErrors []error

	for result := range results {
		if result.err == nil {
			continue
		}

		pullErrors = append(
			pullErrors,
			fmt.Errorf(
				"%s: %w",
				result.image,
				result.err,
			),
		)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	if len(pullErrors) > 0 {
		return fmt.Errorf(
			"pull Docker images: %w",
			errors.Join(
				pullErrors...,
			),
		)
	}

	return nil
}
