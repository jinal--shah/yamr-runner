package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoad(t *testing.T) {
	path := writeYAML(t, `
foo:
  bar: baz
numbers:
  - 1
  - 2
`)

	document, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	root := document.Content[0]

	if got := root.Kind; got != yaml.MappingNode {
		t.Fatalf("root kind = %v, want MappingNode", got)
	}

	if got := mappingValue(root, "foo").Kind; got != yaml.MappingNode {
		t.Fatalf("foo kind = %v, want MappingNode", got)
	}

	if got := mappingValue(root, "foo").Content[1].Value; got != "baz" {
		t.Fatalf("foo.bar = %q, want %q", got, "baz")
	}
}

func TestLoadRejectsNonMappingRoot(t *testing.T) {
	path := writeYAML(t, `
- foo
- bar
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() succeeded, want error")
	}
}

func TestLoadRejectsInvalidYAML(t *testing.T) {
	path := writeYAML(t, `
foo: [
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() succeeded, want error")
	}
}

func TestMergeRecursivelyMergesMappings(t *testing.T) {
	base := parseDocument(t, `
foo:
  one: original
  two: retained
bar: original
`)

	overlay := parseDocument(t, `
foo:
  one: replaced
  three: added
bar: replaced
`)

	merged, err := Merge(base, overlay)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	root := merged.Content[0]

	foo := mappingValue(root, "foo")

	assertMappingScalar(t, foo, "one", "replaced")
	assertMappingScalar(t, foo, "two", "retained")
	assertMappingScalar(t, foo, "three", "added")
	assertMappingScalar(t, root, "bar", "replaced")
}

func TestMergeSequenceReplacesExistingSequence(t *testing.T) {
	base := parseDocument(t, `
sources:
  - one
  - two
`)

	overlay := parseDocument(t, `
sources:
  - three
`)

	merged, err := Merge(base, overlay)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	sources := mappingValue(merged.Content[0], "sources")

	if sources.Kind != yaml.SequenceNode {
		t.Fatalf("sources kind = %v, want SequenceNode", sources.Kind)
	}

	if len(sources.Content) != 1 {
		t.Fatalf("sources contains %d values, want 1", len(sources.Content))
	}

	if sources.Content[0].Value != "three" {
		t.Fatalf("sources[0] = %q, want %q", sources.Content[0].Value, "three")
	}
}

func TestMergeScalarReplacesExistingScalar(t *testing.T) {
	base := parseDocument(t, `
foo: original
`)

	overlay := parseDocument(t, `
foo: replacement
`)

	merged, err := Merge(base, overlay)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	assertMappingScalar(t, merged.Content[0], "foo", "replacement")
}

func TestMergeMappingReplacesScalar(t *testing.T) {
	base := parseDocument(t, `
foo: original
`)

	overlay := parseDocument(t, `
foo:
  nested: value
`)

	merged, err := Merge(base, overlay)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	foo := mappingValue(merged.Content[0], "foo")

	if foo.Kind != yaml.MappingNode {
		t.Fatalf("foo kind = %v, want MappingNode", foo.Kind)
	}

	assertMappingScalar(t, foo, "nested", "value")
}

func TestMergeScalarReplacesMapping(t *testing.T) {
	base := parseDocument(t, `
foo:
  nested: original
`)

	overlay := parseDocument(t, `
foo: replacement
`)

	merged, err := Merge(base, overlay)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	assertMappingScalar(t, merged.Content[0], "foo", "replacement")
}

func TestMergeDoesNotModifyBase(t *testing.T) {
	base := parseDocument(t, `
foo:
  value: original
`)

	overlay := parseDocument(t, `
foo:
  value: replacement
`)

	originalBase := yamlString(t, base)

	if _, err := Merge(base, overlay); err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	if got := yamlString(t, base); got != originalBase {
		t.Fatalf("base was modified:\noriginal:\n%s\nafter:\n%s", originalBase, got)
	}
}

func TestMergeDoesNotModifyOverlay(t *testing.T) {
	base := parseDocument(t, `
foo:
  value: replacement
`)

	overlay := parseDocument(t, `
foo:
  value: original
`)

	originalOverlay := yamlString(t, overlay)

	if _, err := Merge(base, overlay); err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	if got := yamlString(t, overlay); got != originalOverlay {
		t.Fatalf(
			"overlay was modified:\noriginal:\n%s\nafter:\n%s",
			originalOverlay,
			got,
		)
	}
}

func TestMergePreservesScalarLexicalValue(t *testing.T) {
	base := parseDocument(t, `
env:
  VALUE: original
`)

	overlay := parseDocument(t, `
env:
  INTEGER: 00123
  FLOAT: 10.00
  BOOLEAN: true
`)

	merged, err := Merge(base, overlay)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	env := mappingValue(merged.Content[0], "env")

	assertMappingScalar(t, env, "INTEGER", "00123")
	assertMappingScalar(t, env, "FLOAT", "10.00")
	assertMappingScalar(t, env, "BOOLEAN", "true")
}

func TestMergeAddsNewNestedMap(t *testing.T) {
	base := parseDocument(t, `
existing:
  value: one
`)

	overlay := parseDocument(t, `
new:
  nested:
    value: two
`)

	merged, err := Merge(base, overlay)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	newValue := mappingValue(merged.Content[0], "new")
	nested := mappingValue(newValue, "nested")

	assertMappingScalar(t, nested, "value", "two")
}

func parseDocument(t *testing.T, source string) *yaml.Node {
	t.Helper()

	var document yaml.Node
	if err := yaml.Unmarshal([]byte(source), &document); err != nil {
		t.Fatalf("parsing test YAML: %v", err)
	}

	return &document
}

func writeYAML(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")

	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing test YAML: %v", err)
	}

	return path
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}

	panic("mapping key not found: " + key)
}

func assertMappingScalar(t *testing.T, mapping *yaml.Node, key, want string) {
	t.Helper()

	value := mappingValue(mapping, key)

	if value.Kind != yaml.ScalarNode {
		t.Fatalf(
			"%s kind = %v, want ScalarNode",
			key,
			value.Kind,
		)
	}

	if value.Value != want {
		t.Fatalf(
			"%s = %q, want %q",
			key,
			value.Value,
			want,
		)
	}
}

func yamlString(t *testing.T, document *yaml.Node) string {
	t.Helper()

	data, err := yaml.Marshal(document)
	if err != nil {
		t.Fatalf("marshalling YAML: %v", err)
	}

	return string(data)
}
