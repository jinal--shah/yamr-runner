package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

type Result struct {
	ExitCode int  // what did the container return? Always check result.Ran first, as 0 is the default value for int
	Ran      bool // did the docker process run successfully (even if the container returns non-zero)?
}

func (r Result) Successful() bool {
	return r.Ran && r.ExitCode == 0
}

// Run() an already built docker command (see Command struct in docker/docker.go)
// stdout and stderr are supplied by the caller so the runner layer can
// later decide whether output goes to the terminal, files, or elsewhere.
func Run(
	ctx context.Context,
	command Command,
	stdout io.Writer,
	stderr io.Writer,
) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf(
			"context must not be nil",
		)
	}

	args := command.Args()

	cmd := exec.CommandContext(
		ctx,
		"docker",
		args...,
	)

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if err == nil {
		return Result{
			ExitCode: 0,
			Ran:      true,
		}, nil
	}

	// Keep CTRL-C distinct from an ordinary Docker failure.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Result{}, ctxErr
	}

	var exitError *exec.ExitError

	if errors.As(
		err,
		&exitError,
	) {
		return Result{
			ExitCode: exitError.ExitCode(),
			Ran:      true,
		}, nil
	}

	// The Docker CLI itself could not be executed.
	return Result{}, fmt.Errorf(
		"execute docker: %w",
		err,
	)
}
