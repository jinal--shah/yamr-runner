package docker

import (
	"reflect"
	"strings"
	"testing"

	"jinal--shah/yamr-runner/internal/action"

	"gopkg.in/yaml.v3"
)

func TestCompile(t *testing.T) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    run:
      docker:
        image: propero/yamr:candidate
        user_group: 501:20
        entrypoint:
          - yamr
        cmd_opts:
          - -c
          - /conf/yamr.yaml
          - -o
          - /dev/null
          - --
        cmd_sources:
          - source1.yaml
          - source2.yaml
        work_dir: /sources
        mounts:
          - /repo:/repo
          - /sources:/sources
`)

	a.Env = map[string]string{
		"ENTITY":   "m4m",
		"PLATFORM": "polaris",
	}

	got, err := Compile(a)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	want := Command{
		Image:      "propero/yamr:candidate",
		Entrypoint: "yamr",
		CmdOpts: []string{
			"-c",
			"/conf/yamr.yaml",
			"-o",
			"/dev/null",
			"--",
		},
		CmdSources: []string{
			"source1.yaml",
			"source2.yaml",
		},
		UserGroup: "501:20",
		WorkDir:   "/sources",
		Mounts: []string{
			"/repo:/repo",
			"/sources:/sources",
		},
		Env: map[string]string{
			"ENTITY":   "m4m",
			"PLATFORM": "polaris",
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf(
			"Compile() = %#v\nwant %#v",
			got,
			want,
		)
	}
}

func TestCompileMinimalConfiguration(t *testing.T) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    run:
      docker:
        image: propero/yamr:candidate
        entrypoint: [yamr]
        cmd_sources:
          - source.yaml
`)

	got, err := Compile(a)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if got.Image != "propero/yamr:candidate" {
		t.Fatalf(
			"Image = %q, want %q",
			got.Image,
			"propero/yamr:candidate",
		)
	}

	if got.Entrypoint != "yamr" {
		t.Fatalf(
			"Entrypoint = %q, want %q",
			got.Entrypoint,
			"yamr",
		)
	}

	assertStringSlice(
		t,
		got.CmdSources,
		[]string{"source.yaml"},
	)
}

func TestCompileEntrypointMustContainExactlyOneValue(
	t *testing.T,
) {
	tests := []struct {
		name       string
		entrypoint string
	}{
		{
			name: "multiple",
			entrypoint: `
              - yamr
              - something-else`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := actionFromYAML(t, `
yamr-runner:
  action:
    run:
      docker:
        image: propero/yamr:candidate
        entrypoint: `+tt.entrypoint+`
        cmd_sources:
          - source.yaml
`)

			_, err := Compile(a)
			if err == nil {
				t.Fatal(
					"Compile() succeeded, want error",
				)
			}

			want := "docker.entrypoint should have just the command for docker run --entrypoint"

			if !strings.Contains(err.Error(), want) {
				t.Fatalf(
					"error = %q, want error containing %q",
					err,
					want,
				)
			}
		})
	}
}

func TestCompileRequiresImage(t *testing.T) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    run:
      docker:
        entrypoint: [yamr]
        cmd_sources:
          - source.yaml
`)

	_, err := Compile(a)
	if err == nil {
		t.Fatal("Compile() succeeded, want error")
	}

	assertErrorContains(
		t,
		err,
		"yamr-runner.action.run.docker.image is required",
	)
}

func TestCompileRejectsEmptyImage(t *testing.T) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    run:
      docker:
        image: ""
        entrypoint: [yamr]
        cmd_sources:
          - source.yaml
`)

	_, err := Compile(a)
	if err == nil {
		t.Fatal("Compile() succeeded, want error")
	}

	assertErrorContains(
		t,
		err,
		"yamr-runner.action.run.docker.image must not be empty",
	)
}

func TestCompileEntrypointIsOptional(t *testing.T) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    run:
      docker:
        image: propero/yamr:candidate
        cmd_sources:
          - source.yaml
`)

	_, err := Compile(a)
	if err != nil {
		t.Fatalf("Compile() failed, want success: %q", err)
	}
}

func TestCompileRequiresCmdSources(t *testing.T) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    run:
      docker:
        image: propero/yamr:candidate
        entrypoint: [yamr]
`)

	_, err := Compile(a)
	if err == nil {
		t.Fatal("Compile() succeeded, want error")
	}

	assertErrorContains(
		t,
		err,
		"yamr-runner.action.run.docker.cmd_sources is required",
	)
}

func TestCompileRejectsEmptyCmdSources(t *testing.T) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    run:
      docker:
        image: propero/yamr:candidate
        entrypoint: [yamr]
        cmd_sources: []
`)

	_, err := Compile(a)
	if err == nil {
		t.Fatal("Compile() succeeded, want error")
	}

	assertErrorContains(
		t,
		err,
		"cmd_sources must contain at least one source",
	)
}

func TestCompileRejectsWrongSequenceTypes(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "entrypoint scalar",
			yaml: `
        entrypoint: yamr
        cmd_sources: [source.yaml]`,
			want: "entrypoint must be a sequence",
		},
		{
			name: "cmd opts scalar",
			yaml: `
        entrypoint: [yamr]
        cmd_opts: -c
        cmd_sources: [source.yaml]`,
			want: "cmd_opts must be a sequence",
		},
		{
			name: "cmd sources scalar",
			yaml: `
        entrypoint: [yamr]
        cmd_sources: source.yaml`,
			want: "cmd_sources must be a sequence",
		},
		{
			name: "mounts scalar",
			yaml: `
        entrypoint: [yamr]
        cmd_sources: [source.yaml]
        mounts: /repo:/repo`,
			want: "mounts must be a sequence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := actionFromYAML(t, `
yamr-runner:
  action:
    run:
      docker:
        image: propero/yamr:candidate
`+tt.yaml)

			_, err := Compile(a)
			if err == nil {
				t.Fatal(
					"Compile() succeeded, want error",
				)
			}

			assertErrorContains(t, err, tt.want)
		})
	}
}

func TestCompileClonesEnvironment(t *testing.T) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    run:
      docker:
        image: propero/yamr:candidate
        entrypoint: [yamr]
        cmd_sources: [source.yaml]
`)

	a.Env = map[string]string{
		"ENTITY": "m4m",
	}

	got, err := Compile(a)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	got.Env["ENTITY"] = "changed"
	got.Env["NEW"] = "value"

	if a.Env["ENTITY"] != "m4m" {
		t.Fatalf(
			"action environment was mutated: %q",
			a.Env["ENTITY"],
		)
	}

	if _, ok := a.Env["NEW"]; ok {
		t.Fatal(
			"adding to Command.Env mutated action environment",
		)
	}
}

func TestArgs(t *testing.T) {
	command := Command{
		Image:      "propero/yamr:candidate",
		Entrypoint: "yamr",
		UserGroup:  "501:20",
		WorkDir:    "/sources",
		Env: map[string]string{
			"PLATFORM": "polaris",
			"ENTITY":   "m4m",
		},
		Mounts: []string{
			"/repo:/repo",
			"/sources:/sources",
		},
		CmdOpts: []string{
			"-c",
			"/conf/yamr.yaml",
			"-o",
			"/dev/null",
			"--",
		},
		CmdSources: []string{
			"source1.yaml",
			"source2.yaml",
		},
	}

	got := command.Args()

	want := []string{
		"run",
		"--rm",
		"--user",
		"501:20",
		"--entrypoint",
		"yamr",
		"--workdir",
		"/sources",
		"--env",
		"ENTITY=m4m",
		"--env",
		"PLATFORM=polaris",
		"--volume",
		"/repo:/repo",
		"--volume",
		"/sources:/sources",
		"propero/yamr:candidate",
		"-c",
		"/conf/yamr.yaml",
		"-o",
		"/dev/null",
		"--",
		"source1.yaml",
		"source2.yaml",
	}

	assertStringSlice(t, got, want)
}

func TestArgsMinimal(t *testing.T) {
	command := Command{
		Image:      "propero/yamr:candidate",
		Entrypoint: "yamr",
		CmdSources: []string{
			"source.yaml",
		},
	}

	got := command.Args()

	want := []string{
		"run",
		"--rm",
		"--entrypoint",
		"yamr",
		"propero/yamr:candidate",
		"source.yaml",
	}

	assertStringSlice(t, got, want)
}

func TestArgsEnvironmentIsDeterministic(t *testing.T) {
	command := Command{
		Image:      "image",
		Entrypoint: "yamr",
		Env: map[string]string{
			"Z": "last",
			"A": "first",
			"M": "middle",
		},
		CmdSources: []string{"source.yaml"},
	}

	got := command.Args()

	want := []string{
		"run",
		"--rm",
		"--entrypoint",
		"yamr",
		"--env",
		"A=first",
		"--env",
		"M=middle",
		"--env",
		"Z=last",
		"image",
		"source.yaml",
	}

	assertStringSlice(t, got, want)
}

func actionFromYAML(
	t *testing.T,
	value string,
) *action.Action {
	t.Helper()

	var document yaml.Node

	if err := yaml.Unmarshal(
		[]byte(value),
		&document,
	); err != nil {
		t.Fatalf(
			"yaml.Unmarshal() error = %v",
			err,
		)
	}

	return &action.Action{
		ActionFile: "/repo/example/.yamr.yaml",
		ActionDir:  "/repo/example",
		Config:     &document,
	}
}

func assertStringSlice(
	t *testing.T,
	got []string,
	want []string,
) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf(
			"got:\n%q\nwant:\n%q",
			got,
			want,
		)
	}
}

func assertErrorContains(
	t *testing.T,
	err error,
	want string,
) {
	t.Helper()

	if !strings.Contains(err.Error(), want) {
		t.Fatalf(
			"error = %q, want error containing %q",
			err,
			want,
		)
	}
}
