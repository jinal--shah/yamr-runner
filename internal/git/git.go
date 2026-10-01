package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Repository struct {
	Root string
}

type Status struct {
	Branch string
	SHA    string
	Dirty  bool
	Lines  []string
}

// Discover validates that dir is inside a Git working tree and returns the
// canonical absolute path to the repository root.
func Discover(ctx context.Context, dir string) (Repository, error) {
	root, err := output(
		ctx,
		dir,
		"rev-parse",
		"--show-toplevel",
	)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return Repository{}, fmt.Errorf(
				"directory %q is not inside a Git repository",
				dir,
			)
		}

		return Repository{}, fmt.Errorf("running git: %w", err)
	}

	root = strings.TrimSpace(root)
	if root == "" {
		return Repository{}, fmt.Errorf(
			"git returned an empty repository root for %q",
			dir,
		)
	}

	root, err = canonicalPath(root)
	if err != nil {
		return Repository{}, fmt.Errorf(
			"canonicalising Git repository root %q: %w",
			root,
			err,
		)
	}

	return Repository{
		Root: root,
	}, nil
}

// Inspect returns the current branch, HEAD commit and working-tree status.
//
// Branch is empty for a detached HEAD.
//
// Ignored files are deliberately excluded from Lines and therefore do not
// cause Dirty to be true.
func Inspect(ctx context.Context, repo Repository) (Status, error) {
	if repo.Root == "" {
		return Status{}, errors.New("Git repository root is empty")
	}

	sha, err := output(
		ctx,
		repo.Root,
		"rev-parse",
		"--verify",
		"HEAD",
	)
	if err != nil {
		return Status{}, fmt.Errorf(
			"getting HEAD commit for Git repository %q: %w",
			repo.Root,
			err,
		)
	}

	sha = strings.TrimSpace(sha)
	if sha == "" {
		return Status{}, fmt.Errorf(
			"Git repository %q returned an empty HEAD commit",
			repo.Root,
		)
	}

	branch, err := currentBranch(ctx, repo.Root)
	if err != nil {
		return Status{}, err
	}

	statusOutput, err := output(
		ctx,
		repo.Root,
		"status",
		"--porcelain=v1",
		"--untracked-files=all",
	)
	if err != nil {
		return Status{}, fmt.Errorf(
			"getting status for Git repository %q: %w",
			repo.Root,
			err,
		)
	}

	lines := splitLines(statusOutput)

	return Status{
		Branch: branch,
		SHA:    sha,
		Dirty:  len(lines) != 0,
		Lines:  lines,
	}, nil
}

func currentBranch(ctx context.Context, root string) (string, error) {
	branch, err := output(
		ctx,
		root,
		"symbolic-ref",
		"--quiet",
		"--short",
		"HEAD",
	)
	if err == nil {
		return strings.TrimSpace(branch), nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		// A detached HEAD is valid. There simply isn't a branch name.
		return "", nil
	}

	return "", fmt.Errorf(
		"getting current branch for Git repository %q: %w",
		root,
		err,
	)
}

func output(
	ctx context.Context,
	dir string,
	args ...string,
) (string, error) {
	commandArgs := make([]string, 0, len(args)+2)
	commandArgs = append(commandArgs, "-C", dir)
	commandArgs = append(commandArgs, args...)

	cmd := exec.CommandContext(ctx, "git", commandArgs...)

	data, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}

	return filepath.Clean(canonical), nil
}

func splitLines(value string) []string {
	value = strings.TrimRight(value, "\r\n")
	if value == "" {
		return nil
	}

	return strings.Split(value, "\n")
}
