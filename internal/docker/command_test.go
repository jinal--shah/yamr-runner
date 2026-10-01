package docker

import (
	"testing"
)

func TestCommandShellCommand(
	t *testing.T,
) {
	command := Command{
		Image:      "example/image:latest",
		Entrypoint: "yamr",
		UserGroup:  "1000:1000",
		WorkDir:    "/some path/work",
		Env: map[string]string{
			"NORMAL": "value",
			"SECRET": "it's complicated",
		},
		Mounts: []string{
			"/host path:/container path",
		},
		CmdOpts: []string{
			"-c",
			"/some path/config.yaml",
			"--",
		},
		CmdSources: []string{
			"one.yaml",
			"source with spaces.yaml",
		},
	}

	got := command.ShellCommand()

	want := "docker run --rm --user 1000:1000 " +
		"--entrypoint yamr " +
		"--workdir '/some path/work' " +
		"--env NORMAL=value " +
		`--env 'SECRET=it'"'"'s complicated' ` +
		"--volume '/host path:/container path' " +
		"example/image:latest " +
		"-c '/some path/config.yaml' -- " +
		"one.yaml 'source with spaces.yaml'"

	if got != want {
		t.Fatalf(
			"ShellCommand() = %q, want %q",
			got,
			want,
		)
	}
}
