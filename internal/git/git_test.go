package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverFromRepositoryRoot(t *testing.T) {
	repo := newTestRepository(t)

	got, err := Discover(context.Background(), repo)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	want := canonicalTestPath(t, repo)

	if got.Root != want {
		t.Fatalf(
			"Discover().Root = %q, want %q",
			got.Root,
			want,
		)
	}
}

func TestDiscoverFromNestedDirectory(t *testing.T) {
	repo := newTestRepository(t)

	nested := filepath.Join(repo, "one", "two", "three")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(context.Background(), nested)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	want := canonicalTestPath(t, repo)

	if got.Root != want {
		t.Fatalf(
			"Discover().Root = %q, want %q",
			got.Root,
			want,
		)
	}
}

func TestDiscoverOutsideRepository(t *testing.T) {
	dir := t.TempDir()

	_, err := Discover(context.Background(), dir)
	if err == nil {
		t.Fatal("Discover() succeeded, want error")
	}
}

func TestInspectCleanRepository(t *testing.T) {
	repo := newTestRepository(t)

	status := inspectTestRepository(t, repo)

	if status.Dirty {
		t.Fatalf(
			"Dirty = true, want false; lines = %q",
			status.Lines,
		)
	}

	if len(status.Lines) != 0 {
		t.Fatalf(
			"Lines = %q, want none",
			status.Lines,
		)
	}

	if status.SHA == "" {
		t.Fatal("SHA is empty")
	}

	if status.Branch == "" {
		t.Fatal("Branch is empty")
	}
}

func TestInspectModifiedTrackedFile(t *testing.T) {
	repo := newTestRepository(t)

	writeFile(
		t,
		filepath.Join(repo, "tracked.txt"),
		"modified\n",
	)

	status := inspectTestRepository(t, repo)

	if !status.Dirty {
		t.Fatal("Dirty = false, want true")
	}

	assertStatusContains(t, status, " M tracked.txt")
}

func TestInspectStagedChange(t *testing.T) {
	repo := newTestRepository(t)

	writeFile(
		t,
		filepath.Join(repo, "staged.txt"),
		"staged\n",
	)

	runGit(t, repo, "add", "staged.txt")

	status := inspectTestRepository(t, repo)

	if !status.Dirty {
		t.Fatal("Dirty = false, want true")
	}

	assertStatusContains(t, status, "A  staged.txt")
}

func TestInspectDeletedTrackedFile(t *testing.T) {
	repo := newTestRepository(t)

	if err := os.Remove(filepath.Join(repo, "tracked.txt")); err != nil {
		t.Fatal(err)
	}

	status := inspectTestRepository(t, repo)

	if !status.Dirty {
		t.Fatal("Dirty = false, want true")
	}

	assertStatusContains(t, status, " D tracked.txt")
}

func TestInspectUntrackedFile(t *testing.T) {
	repo := newTestRepository(t)

	writeFile(
		t,
		filepath.Join(repo, "untracked.txt"),
		"untracked\n",
	)

	status := inspectTestRepository(t, repo)

	if !status.Dirty {
		t.Fatal("Dirty = false, want true")
	}

	assertStatusContains(t, status, "?? untracked.txt")
}

func TestInspectIgnoredFileDoesNotMakeRepositoryDirty(t *testing.T) {
	repo := newTestRepository(t)

	writeFile(
		t,
		filepath.Join(repo, "ignored.tmp"),
		"ignored\n",
	)

	status := inspectTestRepository(t, repo)

	if status.Dirty {
		t.Fatalf(
			"Dirty = true, want false; lines = %q",
			status.Lines,
		)
	}
}

func TestInspectIgnoredAndUntrackedFiles(t *testing.T) {
	repo := newTestRepository(t)

	writeFile(
		t,
		filepath.Join(repo, "ignored.tmp"),
		"ignored\n",
	)

	writeFile(
		t,
		filepath.Join(repo, "visible.txt"),
		"visible\n",
	)

	status := inspectTestRepository(t, repo)

	if !status.Dirty {
		t.Fatal("Dirty = false, want true")
	}

	assertStatusContains(t, status, "?? visible.txt")

	for _, line := range status.Lines {
		if strings.Contains(line, "ignored.tmp") {
			t.Fatalf(
				"ignored file appears in status: %q",
				line,
			)
		}
	}
}

func TestInspectDetachedHEAD(t *testing.T) {
	repo := newTestRepository(t)

	sha := strings.TrimSpace(
		runGit(t, repo, "rev-parse", "HEAD"),
	)

	runGit(t, repo, "checkout", "--detach", "--quiet", sha)

	status := inspectTestRepository(t, repo)

	if status.Branch != "" {
		t.Fatalf(
			"Branch = %q, want empty for detached HEAD",
			status.Branch,
		)
	}

	if status.SHA != sha {
		t.Fatalf(
			"SHA = %q, want %q",
			status.SHA,
			sha,
		)
	}
}

func TestInspectRepositoryWithoutCommitFails(t *testing.T) {
	repo := t.TempDir()

	runGit(t, repo, "init", "--quiet")

	discovered, err := Discover(context.Background(), repo)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	_, err = Inspect(context.Background(), discovered)
	if err == nil {
		t.Fatal("Inspect() succeeded, want error")
	}
}

func newTestRepository(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()

	runGit(t, repo, "init", "--quiet")

	// Keep the tests independent of the developer's global Git identity.
	runGit(t, repo, "config", "user.name", "yamr-runner test")
	runGit(t, repo, "config", "user.email", "yamr-runner@example.invalid")

	writeFile(
		t,
		filepath.Join(repo, ".gitignore"),
		"*.tmp\n",
	)

	writeFile(
		t,
		filepath.Join(repo, "tracked.txt"),
		"original\n",
	)

	runGit(t, repo, "add", ".gitignore", "tracked.txt")
	runGit(t, repo, "commit", "--quiet", "-m", "initial commit")

	return repo
}

func inspectTestRepository(t *testing.T, repo string) Status {
	t.Helper()

	discovered, err := Discover(context.Background(), repo)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	status, err := Inspect(context.Background(), discovered)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}

	return status
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	commandArgs := append([]string{"-C", dir}, args...)

	cmd := exec.Command("git", commandArgs...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"git %s failed: %v\n%s",
			strings.Join(args, " "),
			err,
			output,
		)
	}

	return string(output)
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.WriteFile(
		path,
		[]byte(contents),
		0o600,
	); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func canonicalTestPath(t *testing.T, path string) string {
	t.Helper()

	path, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("filepath.Abs(%q): %v", path, err)
	}

	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%q): %v", path, err)
	}

	return filepath.Clean(path)
}

func assertStatusContains(
	t *testing.T,
	status Status,
	want string,
) {
	t.Helper()

	for _, line := range status.Lines {
		if line == want {
			return
		}
	}

	t.Fatalf(
		"status lines %q do not contain %q",
		status.Lines,
		want,
	)
}
