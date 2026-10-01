package sources

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLabels(t *testing.T) {
	path := writeLabelsFile(t, `
vpc-platform:
  - aws_account/$env.ENTITY$.yaml
  - platform/$env.PLATFORM$/common.yaml
  - platform/$env.PLATFORM$/$env.STACK$.yaml

other:
  - foo.yaml
`)

	labels, err := LoadLabels(path)
	if err != nil {
		t.Fatalf("LoadLabels() error = %v", err)
	}

	got, err := labels.Lookup("vpc-platform")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}

	want := []string{
		"aws_account/$env.ENTITY$.yaml",
		"platform/$env.PLATFORM$/common.yaml",
		"platform/$env.PLATFORM$/$env.STACK$.yaml",
	}

	assertStrings(t, got, want)
}

func TestLoadLabelsPreservesTokens(t *testing.T) {
	path := writeLabelsFile(t, `
example:
  - $env.ENTITY$.yaml
  - $action_dir$/foo.yaml
`)

	labels, err := LoadLabels(path)
	if err != nil {
		t.Fatalf("LoadLabels() error = %v", err)
	}

	got, err := labels.Lookup("example")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}

	want := []string{
		"$env.ENTITY$.yaml",
		"$action_dir$/foo.yaml",
	}

	assertStrings(t, got, want)
}

func TestLoadLabelsRejectsInvalidYAML(t *testing.T) {
	path := writeLabelsFile(t, `
foo: [
`)

	_, err := LoadLabels(path)
	if err == nil {
		t.Fatal("LoadLabels() succeeded, want error")
	}
}

func TestLoadLabelsRejectsNonMappingRoot(t *testing.T) {
	path := writeLabelsFile(t, `
- foo.yaml
- bar.yaml
`)

	_, err := LoadLabels(path)
	if err == nil {
		t.Fatal("LoadLabels() succeeded, want error")
	}
}

func TestLoadLabelsRejectsNonSequenceValue(t *testing.T) {
	path := writeLabelsFile(t, `
vpc-platform: foo.yaml
`)

	_, err := LoadLabels(path)
	if err == nil {
		t.Fatal("LoadLabels() succeeded, want error")
	}

	if !strings.Contains(err.Error(), "must contain a sequence") {
		t.Fatalf(
			"error = %q, want sequence error",
			err,
		)
	}
}

func TestLoadLabelsRejectsNonStringSequenceValue(t *testing.T) {
	path := writeLabelsFile(t, `
vpc-platform:
  - foo.yaml
  - 123
`)

	_, err := LoadLabels(path)
	if err == nil {
		t.Fatal("LoadLabels() succeeded, want error")
	}

	if !strings.Contains(err.Error(), "non-string value") {
		t.Fatalf(
			"error = %q, want non-string error",
			err,
		)
	}
}

func TestLoadLabelsAllowsEmptySequence(t *testing.T) {
	path := writeLabelsFile(t, `
empty: []
`)

	labels, err := LoadLabels(path)
	if err != nil {
		t.Fatalf("LoadLabels() error = %v", err)
	}

	got, err := labels.Lookup("empty")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}

	if len(got) != 0 {
		t.Fatalf(
			"Lookup() = %q, want empty sequence",
			got,
		)
	}
}

func TestLookupMissingLabelFails(t *testing.T) {
	path := writeLabelsFile(t, `
existing:
  - foo.yaml
`)

	labels, err := LoadLabels(path)
	if err != nil {
		t.Fatalf("LoadLabels() error = %v", err)
	}

	_, err = labels.Lookup("does-not-exist")
	if err == nil {
		t.Fatal("Lookup() succeeded, want error")
	}

	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Fatalf(
			"error = %q, want missing label name",
			err,
		)
	}
}

func TestLookupReturnsCopy(t *testing.T) {
	path := writeLabelsFile(t, `
example:
  - original.yaml
`)

	labels, err := LoadLabels(path)
	if err != nil {
		t.Fatalf("LoadLabels() error = %v", err)
	}

	first, err := labels.Lookup("example")
	if err != nil {
		t.Fatalf("first Lookup() error = %v", err)
	}

	first[0] = "modified.yaml"

	second, err := labels.Lookup("example")
	if err != nil {
		t.Fatalf("second Lookup() error = %v", err)
	}

	if second[0] != "original.yaml" {
		t.Fatalf(
			"Lookup() returned mutable internal data: %q",
			second[0],
		)
	}
}

func TestLoadLabelsMissingFile(t *testing.T) {
	path := filepath.Join(
		t.TempDir(),
		"does-not-exist.yaml",
	)

	_, err := LoadLabels(path)
	if err == nil {
		t.Fatal("LoadLabels() succeeded, want error")
	}
}

func TestLoadLabelsRejectsEmptyLabel(t *testing.T) {
	path := writeLabelsFile(t, `
"":
  - foo.yaml
`)

	_, err := LoadLabels(path)
	if err == nil {
		t.Fatal("LoadLabels() succeeded, want error")
	}
}

func TestLoadLabelsRejectsDuplicateLabel(t *testing.T) {
	path := writeLabelsFile(t, `
foo:
  - one.yaml

foo:
  - two.yaml
`)

	_, err := LoadLabels(path)
	if err == nil {
		t.Fatal("LoadLabels() succeeded, want error")
	}

	if !strings.Contains(err.Error(), "duplicate label") {
		t.Fatalf(
			"error = %q, want duplicate label error",
			err,
		)
	}
}

func writeLabelsFile(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(
		t.TempDir(),
		"yamr-source-labels.yaml",
	)

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

	return path
}

func assertStrings(
	t *testing.T,
	got []string,
	want []string,
) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf(
			"length = %d, want %d\ngot:  %q\nwant: %q",
			len(got),
			len(want),
			got,
			want,
		)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf(
				"[%d] = %q, want %q",
				i,
				got[i],
				want[i],
			)
		}
	}
}
