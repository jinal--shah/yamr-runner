package docker

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestPullDockerImage(
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

		return []byte(
			"pulled",
		), nil
	}

	err := pull(
		context.Background(),
		"alpine:3.22",
		run,
	)
	if err != nil {
		t.Fatalf(
			"pull() error = %v, want nil",
			err,
		)
	}

	if gotName != "docker" {
		t.Fatalf(
			"command = %q, want %q",
			gotName,
			"docker",
		)
	}

	wantArgs := []string{
		"pull",
		"alpine:3.22",
	}

	if !reflect.DeepEqual(
		gotArgs,
		wantArgs,
	) {
		t.Fatalf(
			"args = %#v, want %#v",
			gotArgs,
			wantArgs,
		)
	}
}

func TestPullDockerImageFailure(
	t *testing.T,
) {
	run := func(
		ctx context.Context,
		name string,
		args ...string,
	) ([]byte, error) {
		return []byte(
				"manifest unknown: manifest unknown\n",
			), errors.New(
				"exit status 1",
			)
	}

	err := pull(
		context.Background(),
		"example.invalid/missing:dead",
		run,
	)
	if err == nil {
		t.Fatal(
			"pull() error = nil, want error",
		)
	}

	want := `pull Docker image "example.invalid/missing:dead": manifest unknown: manifest unknown`

	if err.Error() != want {
		t.Fatalf(
			"pull() error = %q, want %q",
			err,
			want,
		)
	}
}

func TestPullDockerImageFailureWithoutOutput(
	t *testing.T,
) {
	commandErr := errors.New(
		"pull failed",
	)

	run := func(
		ctx context.Context,
		name string,
		args ...string,
	) ([]byte, error) {
		return nil, commandErr
	}

	err := pull(
		context.Background(),
		"alpine:3.22",
		run,
	)

	if !errors.Is(
		err,
		commandErr,
	) {
		t.Fatalf(
			"pull() error = %v, want wrapped command error",
			err,
		)
	}
}

func TestPullDockerImageContextCancelled(
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

		return nil, nil
	}

	err := pull(
		ctx,
		"alpine:3.22",
		run,
	)

	if !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf(
			"pull() error = %v, want context.Canceled",
			err,
		)
	}

	if runCalled {
		t.Fatal(
			"command runner called with cancelled context",
		)
	}
}
