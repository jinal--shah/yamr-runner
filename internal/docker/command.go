package docker

import (
	"sort"
	"strings"
)

type Command struct {
	Image      string
	Entrypoint string
	CmdOpts    []string
	CmdSources []string
	UserGroup  string
	WorkDir    string
	Mounts     []string
	Env        map[string]string
}

// Args returns the arguments to pass to the docker executable.
//
// The returned slice does not contain "docker" itself.
func (c Command) Args() []string {
	args := []string{
		"run",
		"--rm",
	}

	if c.UserGroup != "" {
		args = append(
			args,
			"--user",
			c.UserGroup,
		)
	}

	args = append(
		args,
		"--entrypoint",
		c.Entrypoint,
	)

	if c.WorkDir != "" {
		args = append(
			args,
			"--workdir",
			c.WorkDir,
		)
	}

	// Map iteration order is deliberately undefined in Go.
	// Sorting gives us deterministic command lines and tests.
	envKeys := make([]string, 0, len(c.Env))
	for key := range c.Env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)

	for _, key := range envKeys {
		args = append(
			args,
			"--env",
			key+"="+c.Env[key],
		)
	}

	for _, mount := range c.Mounts {
		args = append(
			args,
			"--volume",
			mount,
		)
	}

	args = append(args, c.Image)
	args = append(args, c.CmdOpts...)
	args = append(args, c.CmdSources...)

	return args
}

func (c Command) ShellCommand() string {
	args := append(
		[]string{"docker"},
		c.Args()...,
	)

	quoted := make(
		[]string,
		0,
		len(args),
	)

	for _, arg := range args {
		quoted = append(
			quoted,
			shellQuote(arg),
		)
	}

	return strings.Join(
		quoted,
		" ",
	)
}

func shellQuote(
	value string,
) string {
	if value == "" {
		return "''"
	}

	if isShellSafe(value) {
		return value
	}

	return "'" +
		strings.ReplaceAll(
			value,
			"'",
			`'"'"'`,
		) +
		"'"
}

func isShellSafe(
	value string,
) bool {
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case strings.ContainsRune(
			"_@%+=:,./-",
			r,
		):
		default:
			return false
		}
	}

	return true
}
