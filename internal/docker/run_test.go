package docker

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a shell script")
	}

	installFakeDocker(
		t,
		`#!/bin/sh
printf 'stdout-value'
printf 'stderr-value' >&2
exit 0
`,
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	result, err := Run(
		context.Background(),
		Command{
			Image:      "example/image:latest",
			Entrypoint: "yamr",
			CmdOpts: []string{
				"-c",
				"/conf/yamr.yaml",
			},
			CmdSources: []string{
				"/sources/example.yaml",
			},
		},
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf(
			"Run() error = %v",
			err,
		)
	}

	if result.ExitCode != 0 {
		t.Fatalf(
			"ExitCode = %d, want 0",
			result.ExitCode,
		)
	}

	if stdout.String() != "stdout-value" {
		t.Fatalf(
			"stdout = %q, want %q",
			stdout.String(),
			"stdout-value",
		)
	}

	if stderr.String() != "stderr-value" {
		t.Fatalf(
			"stderr = %q, want %q",
			stderr.String(),
			"stderr-value",
		)
	}
}

func TestRunPassesCompiledArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a shell script")
	}

	installFakeDocker(
		t,
		`#!/bin/sh
printf '%s\n' "$@"
`,
	)

	var stdout bytes.Buffer

	command := Command{
		Image:      "example/image:latest",
		Entrypoint: "yamr",
		UserGroup:  "1000:1000",
		WorkDir:    "/sources",
		Mounts: []string{
			"/repo:/repo",
		},
		Env: map[string]string{
			"STACK":  "prod",
			"ENTITY": "m4m",
		},
		CmdOpts: []string{
			"-c",
			"/conf/yamr.yaml",
			"--",
		},
		CmdSources: []string{
			"/sources/one.yaml",
			"/sources/two.yaml",
		},
	}

	result, err := Run(
		context.Background(),
		command,
		&stdout,
		nil,
	)
	if err != nil {
		t.Fatalf(
			"Run() error = %v",
			err,
		)
	}

	if result.ExitCode != 0 {
		t.Fatalf(
			"ExitCode = %d, want 0",
			result.ExitCode,
		)
	}

	got := strings.Split(
		strings.TrimSpace(stdout.String()),
		"\n",
	)

	want := command.Args()

	if len(got) != len(want) {
		t.Fatalf(
			"arguments = %#v\nwant %#v",
			got,
			want,
		)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf(
				"argument %d = %q, want %q\n"+
					"got:  %#v\n"+
					"want: %#v",
				i,
				got[i],
				want[i],
				got,
				want,
			)
		}
	}
}

func TestRunReturnsDockerExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a shell script")
	}

	installFakeDocker(
		t,
		`#!/bin/sh
printf 'container failed' >&2
exit 42
`,
	)

	var stderr bytes.Buffer

	result, err := Run(
		context.Background(),
		Command{
			Image: "example/image:latest",
		},
		nil,
		&stderr,
	)

	if err != nil {
		t.Fatalf(
			"Run() error = %v, want nil",
			err,
		)
	}

	if !result.Ran {
		t.Fatal(
			"Ran = false, want true",
		)
	}

	if result.ExitCode != 42 {
		t.Fatalf(
			"ExitCode = %d, want 42",
			result.ExitCode,
		)
	}

	if result.Successful() {
		t.Fatal(
			"Successful() = true, want false",
		)
	}

	if stderr.String() != "container failed" {
		t.Fatalf(
			"stderr = %q, want %q",
			stderr.String(),
			"container failed",
		)
	}
}

func TestRunCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a shell script")
	}

	installFakeDocker(
		t,
		`#!/bin/sh
sleep 30
`,
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		100*time.Millisecond,
	)
	defer cancel()

	result, err := Run(
		ctx,
		Command{
			Image: "example/image:latest",
		},
		nil,
		nil,
	)

	if err == nil {
		t.Fatal(
			"Run() succeeded, want cancellation error",
		)
	}

	if err != context.DeadlineExceeded {
		t.Fatalf(
			"error = %v, want context.DeadlineExceeded",
			err,
		)
	}

	if result.ExitCode != 0 {
		t.Fatalf(
			"ExitCode = %d, want zero value",
			result.ExitCode,
		)
	}
}

func TestRunRejectsNilContext(t *testing.T) {
	_, err := Run(
		nil,
		Command{
			Image: "example/image:latest",
		},
		nil,
		nil,
	)

	if err == nil {
		t.Fatal(
			"Run() succeeded, want error",
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

func installFakeDocker(
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
		dir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
}
