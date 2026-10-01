package docker

import (
	"context"
	"fmt"
	"strings"
)

func Pull(
	ctx context.Context,
	image string,
) error {
	return pull(
		ctx,
		image,
		runCommand,
	)
}

func pull(
	ctx context.Context,
	image string,
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

	if image == "" {
		return fmt.Errorf(
			"image must not be empty",
		)
	}

	output, err := run(
		ctx,
		"docker",
		"pull",
		image,
	)
	if err == nil {
		return nil
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	message := strings.TrimSpace(
		string(output),
	)

	if message != "" {
		return fmt.Errorf(
			"pull Docker image %q: %s",
			image,
			message,
		)
	}

	return fmt.Errorf(
		"pull Docker image %q: %w",
		image,
		err,
	)
}
