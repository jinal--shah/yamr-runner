package docker

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type commandRunnerFunc func(
	context.Context,
	string,
	...string,
) ([]byte, error)

func Check(
	ctx context.Context,
) error {
	return check(
		ctx,
		runCommand,
	)
}

func runCommand(
	ctx context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	cmd := exec.CommandContext(
		ctx,
		name,
		args...,
	)

	return cmd.CombinedOutput()
}

func check(
	ctx context.Context,
	run commandRunnerFunc,
) error {
	if ctx == nil {
		return fmt.Errorf(
			"context must not be nil",
		)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	output, err := run(
		ctx,
		"docker",
		"version",
		"--format",
		"{{.Server.Version}}",
	)

	if err == nil {
		return nil
	}

	var execErr *exec.Error
	if errors.As(
		err,
		&execErr,
	) {
		return fmt.Errorf(
			"docker CLI is unavailable: %w",
			err,
		)
	}

	message := strings.TrimSpace(
		string(output),
	)

	if message != "" {
		return fmt.Errorf(
			"docker is unavailable: %s",
			message,
		)
	}

	return fmt.Errorf(
		"docker is unavailable: %w",
		err,
	)
}
