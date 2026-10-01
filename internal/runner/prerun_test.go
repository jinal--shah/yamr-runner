package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"jinal--shah/yamr-run/internal/action"

	"gopkg.in/yaml.v3"
)

const (
	SomeFile string = "/some/file.yamr.yaml"
)

func TestCompilePreRun(t *testing.T) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - mv_to_tmp:
          path: /repo/.generated
      - mkdir:
          path: /repo/.generated
          chmod: "0750"
          chown: 123:456
`)

	got, err := CompilePreRun(a)
	if err != nil {
		t.Fatalf(
			"CompilePreRun() error = %v",
			err,
		)
	}

	want := []PreRunOperation{
		MoveToTmpOperation{
			Path: "/repo/.generated",
		},
		MkdirOperation{
			Path: "/repo/.generated",
			Mode: 0o750,
			UID:  123,
			GID:  456,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf(
			"CompilePreRun() = %#v\nwant %#v",
			got,
			want,
		)
	}
}

func TestCompilePreRunAbsent(t *testing.T) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    run: {}
`)

	got, err := CompilePreRun(a)
	if err != nil {
		t.Fatalf(
			"CompilePreRun() error = %v",
			err,
		)
	}

	if got != nil {
		t.Fatalf(
			"CompilePreRun() = %#v, want nil",
			got,
		)
	}
}

func TestCompilePreRunNull(t *testing.T) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run: null
`)

	got, err := CompilePreRun(a)
	if err != nil {
		t.Fatalf(
			"CompilePreRun() error = %v",
			err,
		)
	}

	if got != nil {
		t.Fatalf(
			"CompilePreRun() = %#v, want nil",
			got,
		)
	}
}

func TestCompilePreRunEmptySequence(t *testing.T) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run: []
`)

	got, err := CompilePreRun(a)
	if err != nil {
		t.Fatalf(
			"CompilePreRun() error = %v",
			err,
		)
	}

	if len(got) != 0 {
		t.Fatalf(
			"len(CompilePreRun()) = %d, want 0",
			len(got),
		)
	}
}

func TestCompilePreRunMkdirDefaults(t *testing.T) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - mkdir:
          path: /repo/.generated
`)

	got, err := CompilePreRun(a)
	if err != nil {
		t.Fatalf(
			"CompilePreRun() error = %v",
			err,
		)
	}

	if len(got) != 1 {
		t.Fatalf(
			"len(CompilePreRun()) = %d, want 1",
			len(got),
		)
	}

	mkdir, ok := got[0].(MkdirOperation)
	if !ok {
		t.Fatalf(
			"CompilePreRun()[0] = %T, want MkdirOperation",
			got[0],
		)
	}

	if mkdir.Mode != 0o755 {
		t.Fatalf(
			"Mode = %04o, want 0755",
			mkdir.Mode,
		)
	}

	if mkdir.UID != os.Geteuid() {
		t.Fatalf(
			"UID = %d, want effective UID %d",
			mkdir.UID,
			os.Geteuid(),
		)
	}

	if mkdir.GID != os.Getegid() {
		t.Fatalf(
			"GID = %d, want effective GID %d",
			mkdir.GID,
			os.Getegid(),
		)
	}
}

func TestCompilePreRunMkdirNullDefaults(
	t *testing.T,
) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - mkdir:
          path: /repo/.generated
          chmod: null
          chown: null
`)

	got, err := CompilePreRun(a)

	if err != nil {
		t.Fatalf(
			"Got unexpected error from CompilePreRun %#v",
			err,
		)
	}

	if len(got) != 1 {
		t.Fatalf(
			"len(CompilePreRun()) = %d, want 1",
			len(got),
		)
	}

	mkdir, ok := got[0].(MkdirOperation)
	if !ok {
		t.Fatalf(
			"CompilePreRun()[0] = %T, want MkdirOperation",
			got[0],
		)
	}

	if mkdir.Mode != 0o755 {
		t.Fatalf(
			"Mode = %04o, want 0755",
			mkdir.Mode,
		)
	}

	if mkdir.UID != os.Geteuid() {
		t.Fatalf(
			"UID = %d, want %d",
			mkdir.UID,
			os.Geteuid(),
		)
	}

	if mkdir.GID != os.Getegid() {
		t.Fatalf(
			"GID = %d, want %d",
			mkdir.GID,
			os.Getegid(),
		)
	}
}

func TestCompilePreRunChmodRepresentations(
	t *testing.T,
) {
	tests := []struct {
		name  string
		value string
		want  os.FileMode
	}{
		{
			name:  "quoted leading zero",
			value: `"0755"`,
			want:  0o755,
		},
		{
			name:  "unquoted leading zero",
			value: `0755`,
			want:  0o755,
		},
		{
			name:  "unquoted without leading zero",
			value: `755`,
			want:  0o755,
		},
		{
			name:  "explicit octal prefix",
			value: `0o750`,
			want:  0o750,
		},
		{
			name:  "special bits",
			value: `"1755"`,
			want:  0o1755,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - mkdir:
          path: /repo/output
          chmod: `+tt.value+`
`)

			got, err := CompilePreRun(a)
			if err != nil {
				t.Fatalf(
					"CompilePreRun() error = %v",
					err,
				)
			}
			if len(got) != 1 {
				t.Fatalf(
					"len(CompilePreRun()) = %d, want 1",
					len(got),
				)
			}

			mkdir, ok := got[0].(MkdirOperation)
			if !ok {
				t.Fatalf(
					"CompilePreRun()[0] = %T, want MkdirOperation",
					got[0],
				)
			}

			if mkdir.Mode != tt.want {
				t.Fatalf(
					"Mode = %04o, want %04o",
					mkdir.Mode,
					tt.want,
				)
			}
		})
	}
}

func TestCompilePreRunRejectsInvalidChmod(
	t *testing.T,
) {
	tests := []string{
		`"0899"`,
		`"not-a-mode"`,
		`"10000"`,
		`""`,
	}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - mkdir:
          path: /repo/output
          chmod: `+value+`
`)

			_, err := CompilePreRun(a)
			if err == nil {
				t.Fatal(
					"CompilePreRun() succeeded, want error",
				)
			}

			assertErrorContains(
				t,
				err,
				"chmod",
			)
		})
	}
}

func TestCompilePreRunExplicitChown(
	t *testing.T,
) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - mkdir:
          path: /repo/output
          chown: 501:20
`)

	got, err := CompilePreRun(a)
	if err != nil {
		t.Fatalf(
			"CompilePreRun() error = %v",
			err,
		)
	}

	if len(got) != 1 {
		t.Fatalf(
			"len(CompilePreRun()) = %d, want 1",
			len(got),
		)
	}

	mkdir, ok := got[0].(MkdirOperation)
	if !ok {
		t.Fatalf(
			"CompilePreRun()[0] = %T, want MkdirOperation",
			got[0],
		)
	}

	if mkdir.UID != 501 {
		t.Fatalf(
			"UID = %d, want 501",
			mkdir.UID,
		)
	}

	if mkdir.GID != 20 {
		t.Fatalf(
			"GID = %d, want 20",
			mkdir.GID,
		)
	}
}

func TestCompilePreRunRejectsInvalidChown(
	t *testing.T,
) {
	tests := []string{
		`root:root`,
		`123`,
		`-1:100`,
		`100:-1`,
		`abc:100`,
		`100:abc`,
		`":"`,
	}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - mkdir:
          path: /repo/output
          chown: `+value+`
`)

			_, err := CompilePreRun(a)
			if err == nil {
				t.Fatal(
					"CompilePreRun() succeeded, want error",
				)
			}
		})
	}
}

func TestCompilePreRunRequiresAbsolutePaths(
	t *testing.T,
) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "mkdir",
			body: `
      - mkdir:
          path: relative/path`,
		},
		{
			name: "mv_to_tmp",
			body: `
      - mv_to_tmp:
          path: relative/path`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
`+tt.body)

			_, err := CompilePreRun(a)
			if err == nil {
				t.Fatal(
					"CompilePreRun() succeeded, want error",
				)
			}

			assertErrorContains(
				t,
				err,
				"must be absolute",
			)
		})
	}
}

func TestCompilePreRunCleansPaths(
	t *testing.T,
) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - mkdir:
          path: /repo/foo/../bar//output
`)

	got, err := CompilePreRun(a)
	if err != nil {
		t.Fatalf(
			"CompilePreRun() error = %v",
			err,
		)
	}
	if len(got) != 1 {
		t.Fatalf(
			"len(CompilePreRun()) = %d, want 1",
			len(got),
		)
	}

	mkdir, ok := got[0].(MkdirOperation)
	if !ok {
		t.Fatalf(
			"CompilePreRun()[0] = %T, want MkdirOperation",
			got[0],
		)
	}

	want := filepath.Clean(
		"/repo/foo/../bar//output",
	)

	if mkdir.Path != want {
		t.Fatalf(
			"Path = %q, want %q",
			mkdir.Path,
			want,
		)
	}
}

func TestCompilePreRunRequiresPath(
	t *testing.T,
) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "mkdir",
			body: `
      - mkdir:
          chmod: "0755"`,
		},
		{
			name: "mv_to_tmp",
			body: `
      - mv_to_tmp: {}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
`+tt.body)

			_, err := CompilePreRun(a)
			if err == nil {
				t.Fatal(
					"CompilePreRun() succeeded, want error",
				)
			}

			assertErrorContains(
				t,
				err,
				"path is required",
			)
		})
	}
}

func TestCompilePreRunRejectsUnknownOptions(
	t *testing.T,
) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - mkdir:
          path: /repo/output
          something_else: value
`)

	_, err := CompilePreRun(a)
	if err == nil {
		t.Fatal(
			"CompilePreRun() succeeded, want error",
		)
	}

	assertErrorContains(
		t,
		err,
		`unknown option "something_else"`,
	)
}

func TestCompilePreRunRejectsUnknownMoveOption(
	t *testing.T,
) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - mv_to_tmp:
          path: /repo/output
          chmod: "0755"
`)

	_, err := CompilePreRun(a)
	if err == nil {
		t.Fatal(
			"CompilePreRun() succeeded, want error",
		)
	}

	assertErrorContains(
		t,
		err,
		`unknown option "chmod"`,
	)
}

func TestCompilePreRunRejectsMultipleOperationsInItem(
	t *testing.T,
) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - mkdir:
          path: /repo/output
        mv_to_tmp:
          path: /repo/old
`)

	_, err := CompilePreRun(a)
	if err == nil {
		t.Fatal(
			"CompilePreRun() succeeded, want error",
		)
	}

	assertErrorContains(
		t,
		err,
		"must contain exactly one operation",
	)
}

func TestCompilePreRunRejectsUnsupportedOperation(
	t *testing.T,
) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - remove:
          path: /repo/output
`)

	_, err := CompilePreRun(a)
	if err == nil {
		t.Fatal(
			"CompilePreRun() succeeded, want error",
		)
	}

	assertErrorContains(
		t,
		err,
		`unsupported operation "remove"`,
	)
}

func TestCompilePreRunRejectsNonMappingItem(
	t *testing.T,
) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - mkdir /repo/output
`)

	_, err := CompilePreRun(a)
	if err == nil {
		t.Fatal(
			"CompilePreRun() succeeded, want error",
		)
	}

	assertErrorContains(
		t,
		err,
		"must be a mapping",
	)
}

func TestCompilePreRunRejectsNonMappingOperationConfig(
	t *testing.T,
) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      - mkdir: /repo/output
`)

	_, err := CompilePreRun(a)
	if err == nil {
		t.Fatal(
			"CompilePreRun() succeeded, want error",
		)
	}

	assertErrorContains(
		t,
		err,
		"mkdir must be a mapping",
	)
}

func TestCompilePreRunRejectsNonSequence(
	t *testing.T,
) {
	a := actionFromYAML(t, `
yamr-runner:
  action:
    pre_run:
      mkdir:
        path: /repo/output
`)

	_, err := CompilePreRun(a)
	if err == nil {
		t.Fatal(
			"CompilePreRun() succeeded, want error",
		)
	}

	assertErrorContains(
		t,
		err,
		"yamr-runner.action.pre_run must be a sequence",
	)
}

func TestExecutionTempRootIsShared(t *testing.T) {
	execution := NewExecution(nil)

	first, err := execution.TempRoot()
	if err != nil {
		t.Fatalf(
			"TempRoot() error = %v",
			err,
		)
	}

	second, err := execution.TempRoot()
	if err != nil {
		t.Fatalf(
			"TempRoot() second call error = %v",
			err,
		)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(first)
	})

	if first != second {
		t.Fatalf(
			"TempRoot() returned %q then %q",
			first,
			second,
		)
	}
}

func TestExecutionTempRootName(t *testing.T) {
	execution := NewExecution(nil)

	root, err := execution.TempRoot()
	if err != nil {
		t.Fatalf(
			"TempRoot() error = %v",
			err,
		)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(root)
	})

	name := filepath.Base(root)

	pattern := regexp.MustCompile(
		`^yamr-run-` +
			`\d{4}-\d{2}-\d{2}_` +
			`\d{2}_\d{2}_\d{2}_` +
			`\d{3}-.+$`,
	)

	if !pattern.MatchString(name) {
		t.Fatalf(
			"temporary directory name %q "+
				"does not match expected format",
			name,
		)
	}
}

func TestTemporaryDestinationMirrorsAbsolutePath(
	t *testing.T,
) {
	tempRoot := filepath.Join(
		string(filepath.Separator),
		"tmp",
		"yamr-run-example",
	)

	source := filepath.Join(
		string(filepath.Separator),
		"home",
		"user",
		"repo",
		"foo",
		".generated",
	)

	got, err := temporaryDestination(
		tempRoot,
		source,
	)
	if err != nil {
		t.Fatalf(
			"temporaryDestination() error = %v",
			err,
		)
	}

	want := filepath.Join(
		tempRoot,
		"home",
		"user",
		"repo",
		"foo",
		".generated",
	)

	if got != want {
		t.Fatalf(
			"temporaryDestination() = %q, want %q",
			got,
			want,
		)
	}
}

func TestTemporaryDestinationRejectsRelativePath(
	t *testing.T,
) {
	_, err := temporaryDestination(
		"/tmp/example",
		"repo/.generated",
	)
	if err == nil {
		t.Fatal(
			"temporaryDestination() succeeded, want error",
		)
	}

	assertErrorContains(
		t,
		err,
		"must be absolute",
	)
}

func TestTemporaryDestinationRejectsFilesystemRoot(
	t *testing.T,
) {
	_, err := temporaryDestination(
		"/tmp/example",
		string(filepath.Separator),
	)
	if err == nil {
		t.Fatal(
			"temporaryDestination() succeeded, want error",
		)
	}

	assertErrorContains(
		t,
		err,
		"cannot move filesystem root",
	)
}

func TestRunPreRunMoveToTmpMirrorsPath(
	t *testing.T,
) {
	root := t.TempDir()

	source := filepath.Join(
		root,
		"repo",
		"foo",
		".generated",
	)

	if err := os.MkdirAll(
		source,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(source, "example.txt"),
		[]byte("hello"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	execution := NewExecution(nil)

	err := execution.RunPreRun(
		SomeFile,
		[]PreRunOperation{
			MoveToTmpOperation{
				Path: source,
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"RunPreRun() error = %v",
			err,
		)
	}

	tempRoot, err := execution.TempRoot()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tempRoot)
	})

	wantDestination, err := temporaryDestination(
		tempRoot,
		source,
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Lstat(source); !os.IsNotExist(err) {
		t.Fatalf(
			"source still exists; error = %v",
			err,
		)
	}

	content, err := os.ReadFile(
		filepath.Join(
			wantDestination,
			"example.txt",
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	if string(content) != "hello" {
		t.Fatalf(
			"content = %q, want hello",
			content,
		)
	}
}

func TestRunPreRunMultipleMovesShareTempRoot(
	t *testing.T,
) {
	root := t.TempDir()

	first := filepath.Join(
		root,
		"one",
		".generated",
	)

	second := filepath.Join(
		root,
		"two",
		".generated",
	)

	for _, path := range []string{
		first,
		second,
	} {
		if err := os.MkdirAll(
			path,
			0o755,
		); err != nil {
			t.Fatal(err)
		}
	}

	execution := NewExecution(nil)

	err := execution.RunPreRun(
		SomeFile,
		[]PreRunOperation{
			MoveToTmpOperation{
				Path: first,
			},
			MoveToTmpOperation{
				Path: second,
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"RunPreRun() error = %v",
			err,
		)
	}

	tempRoot, err := execution.TempRoot()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tempRoot)
	})

	for _, source := range []string{
		first,
		second,
	} {
		wantDestination, err := temporaryDestination(
			tempRoot,
			source,
		)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := os.Stat(wantDestination); err != nil {
			t.Fatalf(
				"expected moved path %q does not exist: %v",
				wantDestination,
				err,
			)
		}

		if _, err := os.Lstat(source); !os.IsNotExist(err) {
			t.Fatalf(
				"source %q still exists; error = %v",
				source,
				err,
			)
		}
	}

}

func TestRunPreRunRuntimeCollisionLeavesSourceUntouched(
	t *testing.T,
) {
	root := t.TempDir()

	source := filepath.Join(
		root,
		"repo",
		".generated",
	)

	if err := os.MkdirAll(
		source,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	marker := filepath.Join(
		source,
		"marker.txt",
	)

	if err := os.WriteFile(
		marker,
		[]byte("original"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	execution := NewExecution(nil)

	tempRoot, err := execution.TempRoot()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tempRoot)
	})

	destination, err := temporaryDestination(
		tempRoot,
		source,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(
		destination,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	err = execution.RunPreRun(
		SomeFile,
		[]PreRunOperation{
			MoveToTmpOperation{
				Path: source,
			},
		},
	)

	if err == nil {
		t.Fatal(
			"RunPreRun() succeeded, want collision error",
		)
	}

	assertErrorContains(
		t,
		err,
		"already exists",
	)

	// A runtime collision must be detected before the source is
	// modified.
	content, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf(
			"source was not left intact: %v",
			err,
		)
	}

	if string(content) != "original" {
		t.Fatalf(
			"source content = %q, want original",
			content,
		)
	}
}

func TestRunPreRunMoveMissingPathIsNoOp(
	t *testing.T,
) {
	source := filepath.Join(
		t.TempDir(),
		"missing",
	)

	execution := NewExecution(nil)

	err := execution.RunPreRun(
		SomeFile,
		[]PreRunOperation{
			MoveToTmpOperation{
				Path: source,
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"RunPreRun() error = %v",
			err,
		)
	}
	// No source means there was nothing to preserve, so we should
	// not create a temporary directory merely for the no-op.
	if execution.tempRoot != "" {
		t.Fatalf(
			"temp root %q created for missing source",
			execution.tempRoot,
		)
	}
}

func TestRunPreRunMkdir(t *testing.T) {
	root := t.TempDir()

	path := filepath.Join(
		root,
		"a",
		"b",
		"output",
	)

	execution := NewExecution(nil)

	err := execution.RunPreRun(
		SomeFile,
		[]PreRunOperation{
			MkdirOperation{
				Path: path,
				Mode: 0o750,
				UID:  os.Geteuid(),
				GID:  os.Getegid(),
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"RunPreRun() error = %v",
			err,
		)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if !info.IsDir() {
		t.Fatalf(
			"%q is not a directory",
			path,
		)
	}

	if info.Mode().Perm() != 0o750 {
		t.Fatalf(
			"mode = %04o, want 0750",
			info.Mode().Perm(),
		)
	}
}

func TestRunPreRunMkdirCorrectsExistingMode(
	t *testing.T,
) {
	path := filepath.Join(
		t.TempDir(),
		"output",
	)

	if err := os.Mkdir(
		path,
		0o700,
	); err != nil {
		t.Fatal(err)
	}

	execution := NewExecution(nil)

	err := execution.RunPreRun(
		SomeFile,
		[]PreRunOperation{
			MkdirOperation{
				Path: path,
				Mode: 0o755,
				UID:  os.Geteuid(),
				GID:  os.Getegid(),
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"RunPreRun() error = %v",
			err,
		)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o755 {
		t.Fatalf(
			"mode = %04o, want 0755",
			info.Mode().Perm(),
		)
	}
}

func TestRunPreRunMoveThenMkdir(t *testing.T) {
	root := t.TempDir()

	generated := filepath.Join(
		root,
		"repo",
		".generated",
	)

	if err := os.MkdirAll(
		generated,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	old_txt := filepath.Join(generated, "old.txt")
	if err := os.WriteFile(
		old_txt,
		[]byte("old"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	execution := NewExecution(nil)

	err := execution.RunPreRun(
		SomeFile,
		[]PreRunOperation{
			MoveToTmpOperation{
				Path: generated,
			},
			MkdirOperation{
				Path: generated,
				Mode: 0o755,
				UID:  os.Geteuid(),
				GID:  os.Getegid(),
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"RunPreRun() error = %v",
			err,
		)
	}

	info, err := os.Stat(generated)
	if err != nil {
		t.Fatalf(
			"recreated directory missing: %v",
			err,
		)
	}

	if !info.IsDir() {
		t.Fatal(
			"recreated path is not a directory",
		)
	}

	if _, err := os.Stat(
		filepath.Join(
			generated,
			"old.txt",
		),
	); !os.IsNotExist(err) {
		t.Fatal(
			"old file exists in recreated directory",
		)
	}

	tempRoot, err := execution.TempRoot()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempRoot)
	})

	wantDestination, err := temporaryDestination(
		tempRoot,
		old_txt,
	)
	if err != nil {
		t.Fatal(err)
	}
	oldContent, err := os.ReadFile(
		wantDestination,
	)
	if err != nil {
		t.Fatal(err)
	}

	if string(oldContent) != "old" {
		t.Fatalf(
			"moved content = %q, want old",
			oldContent,
		)
	}
}

func TestRunPreRunPreservesOperationOrder(
	t *testing.T,
) {
	root := t.TempDir()

	path := filepath.Join(
		root,
		"output",
	)

	execution := NewExecution(nil)

	err := execution.RunPreRun(
		SomeFile,
		[]PreRunOperation{
			MkdirOperation{
				Path: path,
				Mode: 0o755,
				UID:  os.Geteuid(),
				GID:  os.Getegid(),
			},
			MoveToTmpOperation{
				Path: path,
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"RunPreRun() error = %v",
			err,
		)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf(
			"path exists after mkdir then move; err = %v",
			err,
		)
	}

	tempRoot, err := execution.TempRoot()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tempRoot)
	})
}

func TestRunPreRunEmitsMvToTmpEvent(
	t *testing.T,
) {
	root := t.TempDir()

	source := filepath.Join(
		root,
		"generated",
	)

	if err := os.MkdirAll(
		source,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	writeFile(
		t,
		filepath.Join(source, "test.txt"),
		"test",
	)

	sink := &recordingEventSink{}

	execution := NewExecution(
		sink,
	)

	err := execution.RunPreRun(
		"/repo/action/.yamr.yaml",
		[]PreRunOperation{
			MoveToTmpOperation{
				Path: source,
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"RunPreRun() error = %v",
			err,
		)
	}

	if len(sink.mvToTmp) != 1 {
		t.Fatalf(
			"mv_to_tmp events = %d, want 1",
			len(sink.mvToTmp),
		)
	}

	event := sink.mvToTmp[0]

	if event.ActionFile !=
		"/repo/action/.yamr.yaml" {
		t.Errorf(
			"ActionFile = %q",
			event.ActionFile,
		)
	}

	if event.Source != source {
		t.Errorf(
			"Source = %q, want %q",
			event.Source,
			source,
		)
	}

	if event.Destination == "" {
		t.Error(
			"Destination is empty",
		)
	}

	tempRoot, err := execution.TempRoot()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tempRoot)
	})

	wantDestination, err := temporaryDestination(
		tempRoot,
		source,
	)

	if err != nil {
		t.Fatal(err)
	}

	if event.Destination !=
		wantDestination {
		t.Errorf(
			"event Destination = %q, result Destination = %q",
			event.Destination,
			wantDestination,
		)
	}
}

func TestRunPreRunMissingMvToTmpDoesNotEmitEvent(
	t *testing.T,
) {
	sink := &recordingEventSink{}

	execution := NewExecution(
		sink,
	)

	err := execution.RunPreRun(
		"/repo/action/.yamr.yaml",
		[]PreRunOperation{
			MoveToTmpOperation{
				Path: filepath.Join(
					t.TempDir(),
					"does-not-exist",
				),
			},
		},
	)

	if err != nil {
		t.Fatalf(
			"RunPreRun() error = %v",
			err,
		)
	}

	if len(sink.mvToTmp) != 0 {
		t.Fatalf(
			"mv_to_tmp events = %d, want 0",
			len(sink.mvToTmp),
		)
	}
}

func TestRunPreRunMvToTmpFailureEmitsPreRunFailedEvent(
	t *testing.T,
) {
	root := t.TempDir()

	source := filepath.Join(
		root,
		"source",
	)

	if err := os.MkdirAll(
		source,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	events := &recordingEventSink{}
	execution := NewExecution(events)

	tempRoot, err := execution.TempRoot()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tempRoot)
	})

	destination, err := temporaryDestination(
		tempRoot,
		source,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(
		destination,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	err = execution.RunPreRun(
		SomeFile,
		[]PreRunOperation{
			MoveToTmpOperation{
				Path: source,
			},
		},
	)
	if err == nil {
		t.Fatal(
			"RunPreRun() error = nil, want error",
		)
	}

	if len(events.preRunFailed) != 1 {
		t.Fatalf(
			"len(PreRunFailed) = %d, want 1",
			len(events.preRunFailed),
		)
	}

	event := events.preRunFailed[0]

	if event.ActionFile != SomeFile {
		t.Fatalf(
			"ActionFile = %q, want %q",
			event.ActionFile,
			SomeFile,
		)
	}

	if event.Operation != PreRunOperationMvToTmp {
		t.Fatalf(
			"Operation = %q, want %q",
			event.Operation,
			PreRunOperationMvToTmp,
		)
	}

	if event.Err == nil {
		t.Fatal(
			"Err = nil, want error",
		)
	}

	if _, err := os.Stat(source); err != nil {
		t.Fatalf(
			"source was modified after failed move: %v",
			err,
		)
	}
}

func TestRunPreRunMkdirFailureEmitsPreRunFailedEvent(
	t *testing.T,
) {
	root := t.TempDir()

	path := filepath.Join(
		root,
		"output",
	)

	if err := os.WriteFile(
		path,
		[]byte("not a directory"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	events := &recordingEventSink{}
	execution := NewExecution(events)

	err := execution.RunPreRun(
		SomeFile,
		[]PreRunOperation{
			MkdirOperation{
				Path: path,
				Mode: 0o755,
				UID:  os.Geteuid(),
				GID:  os.Getegid(),
			},
		},
	)
	if err == nil {
		t.Fatal(
			"RunPreRun() error = nil, want error",
		)
	}

	if len(events.preRunFailed) != 1 {
		t.Fatalf(
			"len(PreRunFailed) = %d, want 1",
			len(events.preRunFailed),
		)
	}

	event := events.preRunFailed[0]

	if event.ActionFile != SomeFile {
		t.Fatalf(
			"ActionFile = %q, want %q",
			event.ActionFile,
			SomeFile,
		)
	}

	if event.Operation != PreRunOperationMkdir {
		t.Fatalf(
			"Operation = %q, want %q",
			event.Operation,
			PreRunOperationMkdir,
		)
	}

	if event.Err == nil {
		t.Fatal(
			"Err = nil, want error",
		)
	}
}

func TestRunPreRunMkdirEmitsEventWhenCreated(
	t *testing.T,
) {
	path := filepath.Join(
		t.TempDir(),
		"output",
	)

	events := &recordingEventSink{}
	execution := NewExecution(events)

	err := execution.RunPreRun(
		SomeFile,
		[]PreRunOperation{
			MkdirOperation{
				Path: path,
				Mode: 0o755,
				UID:  os.Geteuid(),
				GID:  os.Getegid(),
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"RunPreRun() error = %v",
			err,
		)
	}

	if len(events.mkdir) != 1 {
		t.Fatalf(
			"len(Mkdir) = %d, want 1",
			len(events.mkdir),
		)
	}

	event := events.mkdir[0]

	if event.ActionFile != SomeFile {
		t.Fatalf(
			"ActionFile = %q, want %q",
			event.ActionFile,
			SomeFile,
		)
	}

	if event.Path != path {
		t.Fatalf(
			"Path = %q, want %q",
			event.Path,
			path,
		)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf(
			"created directory missing: %v",
			err,
		)
	}

	if !info.IsDir() {
		t.Fatalf(
			"%q is not a directory",
			path,
		)
	}
}

func TestRunPreRunExistingMkdirDoesNotEmitEvent(
	t *testing.T,
) {
	path := filepath.Join(
		t.TempDir(),
		"output",
	)

	if err := os.Mkdir(
		path,
		0o700,
	); err != nil {
		t.Fatal(err)
	}

	events := &recordingEventSink{}
	execution := NewExecution(events)

	err := execution.RunPreRun(
		SomeFile,
		[]PreRunOperation{
			MkdirOperation{
				Path: path,
				Mode: 0o750,
				UID:  os.Geteuid(),
				GID:  os.Getegid(),
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"RunPreRun() error = %v",
			err,
		)
	}

	if len(events.mkdir) != 0 {
		t.Fatalf(
			"len(Mkdir) = %d, want 0",
			len(events.mkdir),
		)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o750 {
		t.Fatalf(
			"mode = %04o, want 0750",
			info.Mode().Perm(),
		)
	}
}

func TestCompilePreRunOperationTypes(
	t *testing.T,
) {
	path := t.TempDir()

	a := actionFromYAML(t,
		fmt.Sprintf(`
yamr-runner:
  action:
    pre_run:
      - mv_to_tmp:
          path: %q
      - mkdir:
          path: %q
`,
			filepath.Join(path, "move"),
			filepath.Join(path, "mkdir"),
		),
	)

	got, err := CompilePreRun(a)
	if err != nil {
		t.Fatalf(
			"CompilePreRun() error = %v",
			err,
		)
	}

	if len(got) != 2 {
		t.Fatalf(
			"len(CompilePreRun()) = %d, want 2",
			len(got),
		)
	}

	if _, ok := got[0].(MoveToTmpOperation); !ok {
		t.Fatalf(
			"operation 0 = %T, want MoveToTmpOperation",
			got[0],
		)
	}

	if _, ok := got[1].(MkdirOperation); !ok {
		t.Fatalf(
			"operation 1 = %T, want MkdirOperation",
			got[1],
		)
	}

	if got[0].Type() != PreRunOperationMvToTmp {
		t.Fatalf(
			"operation 0 Type() = %q, want %q",
			got[0].Type(),
			PreRunOperationMvToTmp,
		)
	}

	if got[1].Type() != PreRunOperationMkdir {
		t.Fatalf(
			"operation 1 Type() = %q, want %q",
			got[1].Type(),
			PreRunOperationMkdir,
		)
	}
}

func TestRunPreRunStopsAfterFailure(
	t *testing.T,
) {
	root := t.TempDir()

	source := filepath.Join(
		root,
		"source",
	)

	if err := os.MkdirAll(
		source,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	shouldNotExist := filepath.Join(
		root,
		"should-not-exist",
	)

	execution := NewExecution(nil)

	tempRoot, err := execution.TempRoot()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tempRoot)
	})

	destination, err := temporaryDestination(
		tempRoot,
		source,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Force the first operation to fail deterministically by occupying
	// its temporary destination before RunPreRun begins.
	if err := os.MkdirAll(
		destination,
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	err = execution.RunPreRun(
		SomeFile,
		[]PreRunOperation{
			MoveToTmpOperation{
				Path: source,
			},
			MkdirOperation{
				Path: shouldNotExist,
				Mode: 0o755,
				UID:  os.Geteuid(),
				GID:  os.Getegid(),
			},
		},
	)
	if err == nil {
		t.Fatal(
			"RunPreRun() error = nil, want error",
		)
	}

	if _, err := os.Stat(shouldNotExist); !os.IsNotExist(err) {
		t.Fatalf(
			"operation after failure was executed; "+
				"Stat(%q) error = %v",
			shouldNotExist,
			err,
		)
	}

	// The failed mv_to_tmp must also have left its source intact.
	if _, err := os.Stat(source); err != nil {
		t.Fatalf(
			"failed operation modified source: %v",
			err,
		)
	}
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

func assertErrorContains(
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

func pathWithin(
	parent string,
	child string,
) bool {
	relative, err := filepath.Rel(
		parent,
		child,
	)
	if err != nil {
		return false
	}

	return relative != ".." &&
		!strings.HasPrefix(
			relative,
			".."+string(filepath.Separator),
		)
}

func writeFile(
	t *testing.T,
	path string,
	contents string,
) {
	t.Helper()

	if err := os.WriteFile(
		path,
		[]byte(contents),
		0o600,
	); err != nil {
		t.Fatalf(
			"os.WriteFile(%q): %v",
			path,
			err,
		)
	}
}
