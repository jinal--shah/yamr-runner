package runner

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"jinal--shah/yamr-run/internal/action"
)

func TestCompileOnFailAbsent(t *testing.T) {
	a := onFailTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    run:
      docker:
        image: primary:latest
        cmd_sources:
          - source.yaml
`,
	)

	got, err := CompileOnFail(a)
	if err != nil {
		t.Fatalf(
			"CompileOnFail() error = %v",
			err,
		)
	}

	if got != nil {
		t.Fatalf(
			"CompileOnFail() = %#v, want nil",
			got,
		)
	}
}

func TestCompileOnFailNull(t *testing.T) {
	a := onFailTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    on_fail: null
    run:
      docker:
        image: primary:latest
        cmd_sources:
          - source.yaml
`,
	)

	got, err := CompileOnFail(a)
	if err != nil {
		t.Fatalf(
			"CompileOnFail() error = %v",
			err,
		)
	}

	if got != nil {
		t.Fatalf(
			"CompileOnFail() = %#v, want nil",
			got,
		)
	}
}

func TestCompileOnFail(t *testing.T) {
	actionFile := filepath.Join(
		string(filepath.Separator),
		"repo",
		"example",
		".yamr.yaml",
	)

	actionDir := filepath.Dir(actionFile)

	a := onFailTestAction(
		t,
		actionFile,
		`
yamr-runner:
  action:
    run:
      docker:
        image: primary:latest
        entrypoint:
          - yamr
        cmd_sources:
          - source.yaml

    on_fail:
      run:
        docker:
          image: debug:latest
          user_group: 1000:1000
          entrypoint:
            - yamr
          cmd_opts:
            - -c
            - /conf/debug.yaml
            - --
          cmd_sources:
            - /sources/one.yaml
            - /sources/two.yaml
          work_dir: /sources
          mounts:
            - /repo:/repo
`,
	)

	got, err := CompileOnFail(a)
	if err != nil {
		t.Fatalf(
			"CompileOnFail() error = %v",
			err,
		)
	}

	if got == nil {
		t.Fatal(
			"CompileOnFail() = nil, want plan",
		)
	}

	if got.Docker.Image != "debug:latest" {
		t.Fatalf(
			"Docker.Image = %q, want debug:latest",
			got.Docker.Image,
		)
	}

	if got.Docker.UserGroup != "1000:1000" {
		t.Fatalf(
			"Docker.UserGroup = %q",
			got.Docker.UserGroup,
		)
	}

	if got.Docker.Entrypoint != "yamr" {
		t.Fatalf(
			"Docker.Entrypoint = %q, want yamr",
			got.Docker.Entrypoint,
		)
	}

	if !reflect.DeepEqual(
		got.Docker.CmdOpts,
		[]string{
			"-c",
			"/conf/debug.yaml",
			"--",
		},
	) {
		t.Fatalf(
			"Docker.CmdOpts = %#v",
			got.Docker.CmdOpts,
		)
	}

	if !reflect.DeepEqual(
		got.Docker.CmdSources,
		[]string{
			"/sources/one.yaml",
			"/sources/two.yaml",
		},
	) {
		t.Fatalf(
			"Docker.CmdSources = %#v",
			got.Docker.CmdSources,
		)
	}

	if got.Docker.WorkDir != "/sources" {
		t.Fatalf(
			"Docker.WorkDir = %q",
			got.Docker.WorkDir,
		)
	}

	wantStdout := filepath.Join(
		actionDir,
		".yamr-debug",
		"stdout.log",
	)

	if got.Stdout.Inherit {
		t.Fatal(
			"Stdout.Inherit = true, want false",
		)
	}

	if got.Stdout.Path != wantStdout {
		t.Fatalf(
			"Stdout.Path = %q, want %q",
			got.Stdout.Path,
			wantStdout,
		)
	}

	wantStderr := filepath.Join(
		actionDir,
		".yamr-debug",
		"stderr.log",
	)

	if got.Stderr.Inherit {
		t.Fatal(
			"Stderr.Inherit = true, want false",
		)
	}

	if got.Stderr.Path != wantStderr {
		t.Fatalf(
			"Stderr.Path = %q, want %q",
			got.Stderr.Path,
			wantStderr,
		)
	}
}

func TestCompileOnFailExplicitOutputPaths(
	t *testing.T,
) {
	a := onFailTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    on_fail:
      run:
        docker:
          image: debug:latest
          entrypoint:
            - yamr
          cmd_sources:
            - source.yaml
        stdout: /tmp/yamr-stdout.log
        stderr: /tmp/yamr-stderr.log
`,
	)

	got, err := CompileOnFail(a)
	if err != nil {
		t.Fatalf(
			"CompileOnFail() error = %v",
			err,
		)
	}

	if got.Stdout.Inherit {
		t.Fatal(
			"Stdout.Inherit = true",
		)
	}

	if got.Stdout.Path !=
		filepath.Clean("/tmp/yamr-stdout.log") {
		t.Fatalf(
			"Stdout.Path = %q",
			got.Stdout.Path,
		)
	}

	if got.Stderr.Inherit {
		t.Fatal(
			"Stderr.Inherit = true",
		)
	}

	if got.Stderr.Path !=
		filepath.Clean("/tmp/yamr-stderr.log") {
		t.Fatalf(
			"Stderr.Path = %q",
			got.Stderr.Path,
		)
	}
}

func TestCompileOnFailNullOutputInherits(
	t *testing.T,
) {
	a := onFailTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    on_fail:
      run:
        docker:
          image: debug:latest
          entrypoint:
            - yamr
          cmd_sources:
            - source.yaml
        stdout: null
        stderr: null
`,
	)

	got, err := CompileOnFail(a)
	if err != nil {
		t.Fatalf(
			"CompileOnFail() error = %v",
			err,
		)
	}

	if !got.Stdout.Inherit {
		t.Fatal(
			"Stdout.Inherit = false, want true",
		)
	}

	if got.Stdout.Path != "" {
		t.Fatalf(
			"Stdout.Path = %q, want empty",
			got.Stdout.Path,
		)
	}

	if !got.Stderr.Inherit {
		t.Fatal(
			"Stderr.Inherit = false, want true",
		)
	}

	if got.Stderr.Path != "" {
		t.Fatalf(
			"Stderr.Path = %q, want empty",
			got.Stderr.Path,
		)
	}
}

func TestCompileOnFailRejectsPreRun(
	t *testing.T,
) {
	a := onFailTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    on_fail:
      pre_run:
        - mkdir:
            path: /tmp/debug
      run:
        docker:
          image: debug:latest
          entrypoint:
            - yamr
          cmd_sources:
            - source.yaml
`,
	)

	_, err := CompileOnFail(a)
	if err == nil {
		t.Fatal(
			"CompileOnFail() succeeded, want error",
		)
	}

	assertOnFailErrorContains(
		t,
		err,
		"on_fail.pre_run is not supported",
	)
}

func TestCompileOnFailRequiresRun(t *testing.T) {
	a := onFailTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    on_fail: {}
`,
	)

	_, err := CompileOnFail(a)
	if err == nil {
		t.Fatal(
			"CompileOnFail() succeeded, want error",
		)
	}

	assertOnFailErrorContains(
		t,
		err,
		"on_fail.run is required",
	)
}

func TestCompileOnFailRequiresDocker(
	t *testing.T,
) {
	a := onFailTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    on_fail:
      run:
        stdout: null
`,
	)

	_, err := CompileOnFail(a)
	if err == nil {
		t.Fatal(
			"CompileOnFail() succeeded, want error",
		)
	}

	assertOnFailErrorContains(
		t,
		err,
		"on_fail.run.docker is required",
	)
}

func TestCompileOnFailRejectsInvalidDocker(
	t *testing.T,
) {
	a := onFailTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    on_fail:
      run:
        docker:
          entrypoint:
            - yamr
          cmd_sources:
            - source.yaml
`,
	)

	_, err := CompileOnFail(a)
	if err == nil {
		t.Fatal(
			"CompileOnFail() succeeded, want error",
		)
	}

	assertOnFailErrorContains(
		t,
		err,
		"image",
	)
}

func TestCompileOnFailRejectsRelativeOutputPath(
	t *testing.T,
) {
	a := onFailTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    on_fail:
      run:
        docker:
          image: debug:latest
          entrypoint:
            - yamr
          cmd_sources:
            - source.yaml
        stdout: relative/stdout.log
`,
	)

	_, err := CompileOnFail(a)
	if err == nil {
		t.Fatal(
			"CompileOnFail() succeeded, want error",
		)
	}

	assertOnFailErrorContains(
		t,
		err,
		"must be absolute",
	)
}

func TestCompileOnFailRejectsEmptyOutputPath(
	t *testing.T,
) {
	a := onFailTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    on_fail:
      run:
        docker:
          image: debug:latest
          entrypoint:
            - yamr
          cmd_sources:
            - source.yaml
        stdout: ""
`,
	)

	_, err := CompileOnFail(a)
	if err == nil {
		t.Fatal(
			"CompileOnFail() succeeded, want error",
		)
	}

	assertOnFailErrorContains(
		t,
		err,
		"must not be empty",
	)
}

func TestCompileOnFailRejectsUnknownOption(
	t *testing.T,
) {
	a := onFailTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    on_fail:
      something_else: true
      run:
        docker:
          image: debug:latest
          entrypoint:
            - yamr
          cmd_sources:
            - source.yaml
`,
	)

	_, err := CompileOnFail(a)
	if err == nil {
		t.Fatal(
			"CompileOnFail() succeeded, want error",
		)
	}

	assertOnFailErrorContains(
		t,
		err,
		`unknown option "something_else"`,
	)
}

func TestCompileOnFailRejectsUnknownRunOption(
	t *testing.T,
) {
	a := onFailTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    on_fail:
      run:
        docker:
          image: debug:latest
          entrypoint:
            - yamr
          cmd_sources:
            - source.yaml
        something_else: true
`,
	)

	_, err := CompileOnFail(a)
	if err == nil {
		t.Fatal(
			"CompileOnFail() succeeded, want error",
		)
	}

	assertOnFailErrorContains(
		t,
		err,
		`unknown option "something_else"`,
	)
}

func TestCompileOnFailUsesActionEnvironment(
	t *testing.T,
) {
	a := onFailTestAction(
		t,
		"/repo/example/.yamr.yaml",
		`
yamr-runner:
  action:
    on_fail:
      run:
        docker:
          image: debug:latest
          entrypoint:
            - yamr
          cmd_sources:
            - source.yaml
`,
	)

	a.Env = map[string]string{
		"ENTITY":   "m4m",
		"PLATFORM": "polaris",
	}

	got, err := CompileOnFail(a)
	if err != nil {
		t.Fatalf(
			"CompileOnFail() error = %v",
			err,
		)
	}

	if !reflect.DeepEqual(
		got.Docker.Env,
		a.Env,
	) {
		t.Fatalf(
			"Docker.Env = %#v, want %#v",
			got.Docker.Env,
			a.Env,
		)
	}

	// Ensure the Docker command received its own map rather than a
	// shared mutable reference.
	a.Env["ENTITY"] = "changed"

	if got.Docker.Env["ENTITY"] != "m4m" {
		t.Fatalf(
			"Docker.Env was mutated through Action.Env",
		)
	}
}

func onFailTestAction(
	t *testing.T,
	actionFile string,
	value string,
) *action.Action {
	t.Helper()

	a := actionFromYAML(t, value)

	a.ActionFile = actionFile
	a.ActionDir = filepath.Dir(actionFile)

	return a
}

func assertOnFailErrorContains(
	t *testing.T,
	err error,
	want string,
) {
	t.Helper()

	if !strings.Contains(
		err.Error(),
		want,
	) {
		t.Fatalf(
			"error = %q, want error containing %q",
			err,
			want,
		)
	}
}
