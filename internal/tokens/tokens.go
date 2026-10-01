package tokens

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Phase int

const (
	Immediate Phase = iota
	Final
)

type Context struct {
	// ThisDir is the directory containing the YAML file currently being
	// processed.
	ThisDir string

	RepoRoot      string
	RunDir        string
	YamrSourcesDir string

	// ActionDir is populated during Final resolution.
	ActionDir string

	// Environment available to $env.NAME$.
	// This should already contain the merged YAML yamr-runner.env,
	// with YAML values taking precedence over the process environment.
	Env map[string]string

	// Sources selected for the final action.
	YamrSources          []string
	YamrSourcesFromLabel []string
}

var tokenPattern = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_.]*)\$`)

var immediateTokens = map[string]func(Context) (string, error){
	"uid_me": func(Context) (string, error) {
		return fmt.Sprintf("%d", os.Getuid()), nil
	},
	"gid_me": func(Context) (string, error) {
		return fmt.Sprintf("%d", os.Getgid()), nil
	},
	"repo_root": func(ctx Context) (string, error) {
		return canonicalTokenPath(ctx.RepoRoot)
	},
	"this_dir": func(ctx Context) (string, error) {
		return canonicalTokenPath(ctx.ThisDir)
	},
	"this_dir_rel_to_repo_root": func(ctx Context) (string, error) {
		root, err := canonicalPath(ctx.RepoRoot)
		if err != nil {
			return "", fmt.Errorf("resolving repo root: %w", err)
		}

		dir, err := canonicalPath(ctx.ThisDir)
		if err != nil {
			return "", fmt.Errorf("resolving this directory: %w", err)
		}

		relative, err := filepath.Rel(root, dir)
		if err != nil {
			return "", fmt.Errorf(
				"making %q relative to repo root %q: %w",
				dir,
				root,
				err,
			)
		}

		if relative == "." {
			return "", nil
		}

		return filepath.ToSlash(relative), nil
	},
	"run_dir": func(ctx Context) (string, error) {
		return canonicalTokenPath(ctx.RunDir)
	},
	"yamr_sources_dir": func(ctx Context) (string, error) {
		return canonicalTokenPath(ctx.YamrSourcesDir)
	},
}

func ResolveDocument(document *yaml.Node, ctx Context, phase Phase) error {
	if document == nil {
		return fmt.Errorf("cannot resolve a nil YAML document")
	}

	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return fmt.Errorf("invalid YAML document")
	}

	return resolveNode(document.Content[0], ctx, phase)
}

func resolveNode(node *yaml.Node, ctx Context, phase Phase) error {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := resolveNode(child, ctx, phase); err != nil {
				return err
			}
		}

	case yaml.MappingNode:
		for _, child := range node.Content {
			if err := resolveNode(child, ctx, phase); err != nil {
				return err
			}
		}

	case yaml.SequenceNode:
		for _, child := range node.Content {
			if err := resolveNode(child, ctx, phase); err != nil {
				return err
			}
		}

	case yaml.ScalarNode:
		if node.Tag == "!!str" || node.Tag == "" {
			return resolveScalar(node, ctx, phase)
		}
	}

	return nil
}

func resolveScalar(node *yaml.Node, ctx Context, phase Phase) error {
	matches := tokenPattern.FindAllStringSubmatchIndex(node.Value, -1)
	if len(matches) == 0 {
		return nil
	}

	// A source token represents a sequence rather than a scalar. It is only
	// valid when it constitutes the entire scalar value.
	if len(matches) == 1 && matches[0][0] == 0 && matches[0][1] == len(node.Value) {
		tokenName := node.Value[matches[0][2]:matches[0][3]]

		if tokenName == "yamr_sources" || tokenName == "yamr_sources_from_label" {
			if phase == Immediate {
				return nil
			}

			var values []string
			switch tokenName {
			case "yamr_sources":
				values = ctx.YamrSources
			case "yamr_sources_from_label":
				values = ctx.YamrSourcesFromLabel
			}

			node.Kind = yaml.SequenceNode
			node.Tag = "!!seq"
			node.Value = ""
			node.Content = make([]*yaml.Node, 0, len(values))

			for _, value := range values {
				node.Content = append(node.Content, &yaml.Node{
					Kind:  yaml.ScalarNode,
					Tag:   "!!str",
					Value: value,
					Style: yaml.TaggedStyle,
				})
			}

			return nil
		}
	}

	var result strings.Builder
	last := 0

	for _, match := range matches {
		start, end := match[0], match[1]
		name := node.Value[match[2]:match[3]]

		result.WriteString(node.Value[last:start])

		value, resolved, err := resolveToken(name, ctx, phase)
		if err != nil {
			return err
		}

		if resolved {
			result.WriteString(value)
		} else {
			// Deferred token. Preserve it exactly for the final phase.
			result.WriteString(node.Value[start:end])
		}

		last = end
	}

	result.WriteString(node.Value[last:])
	node.Value = result.String()

	return nil
}

func resolveToken(name string, ctx Context, phase Phase) (string, bool, error) {
	if resolver, ok := immediateTokens[name]; ok {
		value, err := resolver(ctx)
		if err != nil {
			return "", false, fmt.Errorf(
				"resolving token $%s$: %w",
				name,
				err,
			)
		}

		return value, true, nil
	}

	switch {
	case name == "action_dir":
		if phase == Immediate {
			return "", false, nil
		}

		if ctx.ActionDir == "" {
			return "", false, fmt.Errorf(
				"$action_dir$ cannot be resolved without an action directory",
			)
		}

		value, err := canonicalTokenPath(ctx.ActionDir)
		if err != nil {
			return "", false, fmt.Errorf(
				"resolving token $action_dir$: %w",
				err,
			)
		}

		return value, true, nil

	case strings.HasPrefix(name, "env."):
		if phase == Immediate {
			return "", false, nil
		}

		envName := strings.TrimPrefix(name, "env.")
		if envName == "" {
			return "", false, fmt.Errorf("$env.$ has no environment variable name")
		}

		value, ok := ctx.Env[envName]
		if !ok {
			return "", false, fmt.Errorf(
				"environment variable %q referenced by $%s$ is not defined",
				envName,
				name,
			)
		}

		return value, true, nil

	case name == "yamr_sources" || name == "yamr_sources_from_label":
		// These are handled specially in resolveScalar because their
		// replacement is a sequence.
		return "", false, fmt.Errorf(
			"$%s$ must be the entire YAML value",
			name,
		)

	default:
		return "", false, fmt.Errorf("unknown token $%s$", name)
	}
}

func canonicalPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("making %q absolute: %w", path, err)
	}

	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolving symlinks in %q: %w", path, err)
	}

	return filepath.Clean(canonical), nil
}

// canonicalTokenPath returns a canonical absolute path without a leading
// or trailing slash. This allows the caller's YAML to write:
//
//   /$this_dir$/foo
//
// rather than:
//
//   $this_dir$/foo
//
// which would produce a double slash.
func canonicalTokenPath(path string) (string, error) {
	canonical, err := canonicalPath(path)
	if err != nil {
		return "", err
	}

	return strings.Trim(canonical, `/\`), nil
}

func BuildEnvironment(
	document *yaml.Node,
	hostEnv []string,
) (map[string]string, error) {
	env := make(map[string]string)

	// Start with the host environment.
	for _, entry := range hostEnv {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}

		env[key] = value
	}

	if document == nil ||
		document.Kind != yaml.DocumentNode ||
		len(document.Content) != 1 {
		return nil, fmt.Errorf("invalid YAML document")
	}

	root := document.Content[0]

	runner := mappingNodeValue(root, "yamr-runner")
	if runner == nil {
		return env, nil
	}

	if runner.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("yamr-runner must be a mapping")
	}

	yamlEnv := mappingNodeValue(runner, "env")
	if yamlEnv == nil {
		return env, nil
	}

	if yamlEnv.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("yamr-runner.env must be a mapping")
	}

	for i := 0; i < len(yamlEnv.Content); i += 2 {
		keyNode := yamlEnv.Content[i]
		valueNode := yamlEnv.Content[i+1]

		key := keyNode.Value

		if valueNode.Tag == "!!null" {
			delete(env, key)
			continue
		}

		if valueNode.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf(
				"yamr-runner.env.%s must be a scalar or null",
				key,
			)
		}

		// yaml.Node.Value preserves the lexical representation we want:
		//
		//   true  -> "true"
		//   123   -> "123"
		//   10.50 -> "10.50"
		//   ""    -> ""
		env[key] = valueNode.Value
	}

	return env, nil
}

func mappingNodeValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}

	return nil
}

func ResolveString(
	value string,
	ctx Context,
	phase Phase,
) (string, error) {
	node := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}

	if err := resolveScalar(node, ctx, phase); err != nil {
		return "", err
	}

	if node.Kind != yaml.ScalarNode {
		return "", fmt.Errorf(
			"token resolution produced a non-scalar value",
		)
	}

	return node.Value, nil
}
