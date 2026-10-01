package docker

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestCheckDockerAvailable(
	t *testing.T,
) {
	var gotName string
	var gotArgs []string

	run := func(
		ctx context.Context,
		name string,
		args ...string,
	) ([]byte, error) {
		gotName = name
		gotArgs = args

		return []byte("28.5.1\n"), nil
	}

	err := check(
		context.Background(),
		run,
	)
	if err != nil {
		t.Fatalf(
			"check() error = %v, want nil",
			err,
		)
	}

	if gotName != "docker" {
		t.Errorf(
			"command name = %q, want %q",
			gotName,
			"docker",
		)
	}

	wantArgs := []string{
		"version",
		"--format",
		"{{.Server.Version}}",
	}

	if !reflect.DeepEqual(
		gotArgs,
		wantArgs,
	) {
		t.Errorf(
			"command args = %#v, want %#v",
			gotArgs,
			wantArgs,
		)
	}
}

func TestCheckDockerCLIUnavailable(
	t *testing.T,
) {
	run := func(
		ctx context.Context,
		name string,
		args ...string,
	) ([]byte, error) {
		return nil, &exec.Error{
			Name: "docker",
			Err:  exec.ErrNotFound,
		}
	}

	err := check(
		context.Background(),
		run,
	)
	if err == nil {
		t.Fatal(
			"check() error = nil, want error",
		)
	}

	if !strings.Contains(
		err.Error(),
		"docker CLI is unavailable",
	) {
		t.Fatalf(
			"check() error = %q, want Docker CLI unavailable error",
			err,
		)
	}

	var execErr *exec.Error

	if !errors.As(
		err,
		&execErr,
	) {
		t.Fatalf(
			"check() error = %v, want wrapped *exec.Error",
			err,
		)
	}

	if execErr.Name != "docker" {
		t.Errorf(
			"exec error name = %q, want %q",
			execErr.Name,
			"docker",
		)
	}
}

func TestCheckDockerDaemonUnavailable(
	t *testing.T,
) {
	run := func(
		ctx context.Context,
		name string,
		args ...string,
	) ([]byte, error) {
		return []byte(
				"Cannot connect to the Docker daemon at " +
					"unix:///var/run/docker.sock. " +
					"Is the docker daemon running?\n",
			), errors.New(
				"exit status 1",
			)
	}

	err := check(
		context.Background(),
		run,
	)
	if err == nil {
		t.Fatal(
			"check() error = nil, want error",
		)
	}

	want := "" +
		"docker is unavailable: " +
		"Cannot connect to the Docker daemon at " +
		"unix:///var/run/docker.sock. " +
		"Is the docker daemon running?"

	if err.Error() != want {
		t.Fatalf(
			"check() error = %q, want %q",
			err,
			want,
		)
	}
}

func TestCheckDockerFailureWithoutOutput(
	t *testing.T,
) {
	commandErr := errors.New(
		"docker command failed",
	)

	run := func(
		ctx context.Context,
		name string,
		args ...string,
	) ([]byte, error) {
		return nil, commandErr
	}

	err := check(
		context.Background(),
		run,
	)
	if err == nil {
		t.Fatal(
			"check() error = nil, want error",
		)
	}

	want := "docker is unavailable: docker command failed"

	if err.Error() != want {
		t.Fatalf(
			"check() error = %q, want %q",
			err,
			want,
		)
	}

	if !errors.Is(
		err,
		commandErr,
	) {
		t.Fatalf(
			"check() error = %v, want wrapped command error",
			err,
		)
	}
}

func TestCheckDockerContextCancelled(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	runCalled := false

	run := func(
		ctx context.Context,
		name string,
		args ...string,
	) ([]byte, error) {
		runCalled = true

		return nil, ctx.Err()
	}

	err := check(
		ctx,
		run,
	)
	if err == nil {
		t.Fatal(
			"check() error = nil, want error",
		)
	}

	if !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf(
			"check() error = %v, want context.Canceled",
			err,
		)
	}

	if runCalled {
		t.Fatal(
			"command runner called with already-cancelled context",
		)
	}
}
