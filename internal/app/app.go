package app

import (
	"context"
	"fmt"

	"jinal--shah/yamr-run/internal/action"
	"jinal--shah/yamr-run/internal/cli"
	"jinal--shah/yamr-run/internal/config"
	"jinal--shah/yamr-run/internal/discover"
	"jinal--shah/yamr-run/internal/git"
	"jinal--shah/yamr-run/internal/runner"
	"jinal--shah/yamr-run/internal/sources"
)

const DefaultActionFile = ".yamr.yaml"

type BuildOptions struct {
	CLI     cli.Options
	RunDir  string
	HostEnv []string
}

type BuildResult struct {
	Repository         git.Repository
	GitStatus          git.Status
	Plan               runner.Plan
	IgnoredActionFiles []discover.IgnoredActionFile
}

func Build(
	ctx context.Context,
	options BuildOptions,
) (BuildResult, error) {
	if ctx == nil {
		return BuildResult{}, fmt.Errorf(
			"context must not be nil",
		)
	}

	if options.RunDir == "" {
		return BuildResult{}, fmt.Errorf(
			"run directory must not be empty",
		)
	}

	repository, err := git.Discover(
		ctx,
		options.RunDir,
	)
	if err != nil {
		return BuildResult{}, fmt.Errorf(
			"discover git repository: %w",
			err,
		)
	}

	gitStatus, err := git.Inspect(
		ctx,
		repository,
	)
	if err != nil {
		return BuildResult{}, fmt.Errorf(
			"inspect git repository: %w",
			err,
		)
	}

	mainConfig, err := config.Load(
		options.CLI.ConfigFile,
	)
	if err != nil {
		return BuildResult{}, fmt.Errorf(
			"load runner config: %w",
			err,
		)
	}

	defaults, err := config.RunnerConfig(
		mainConfig,
	)
	if err != nil {
		return BuildResult{}, fmt.Errorf(
			"extract yamr-runner defaults: %w",
			err,
		)
	}

	labels, err := sources.LoadLabels(
		options.CLI.YamrSourceLabels,
	)
	if err != nil {
		return BuildResult{}, fmt.Errorf(
			"load yamr source labels: %w",
			err,
		)
	}

	discovery, err := discover.Find(
		discover.Options{
			RunDir:         options.RunDir,
			RepoRoot:       repository.Root,
			YamrSourcesDir: options.CLI.YamrSourcesDir,
			ActionFile:     DefaultActionFile,
			Defaults:       defaults,
		},
	)
	if err != nil {
		return BuildResult{}, fmt.Errorf(
			"discover actions: %w",
			err,
		)
	}

	actions := make(
		[]*action.Action,
		0,
		len(discovery.Candidates),
	)

	for _, candidate := range discovery.Candidates {
		compiled, err := action.Compile(
			action.Options{
				Candidate: candidate,
				Labels:    labels,
				HostEnv:   options.HostEnv,
			},
		)
		if err != nil {
			return BuildResult{}, fmt.Errorf(
				"compile action %q: %w",
				candidate.ActionFile,
				err,
			)
		}

		actions = append(
			actions,
			compiled,
		)
	}

	plan, err := runner.BuildPlan(
		actions,
	)
	if err != nil {
		return BuildResult{}, fmt.Errorf(
			"build execution plan: %w",
			err,
		)
	}

	return BuildResult{
		Repository:         repository,
		GitStatus:          gitStatus,
		Plan:               plan,
		IgnoredActionFiles: discovery.IgnoredActionFiles,
	}, nil
}
