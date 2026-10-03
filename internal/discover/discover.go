package discover

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"jinal--shah/yamr-runner/internal/config"
	"jinal--shah/yamr-runner/internal/tokens"
)

type Options struct {
	RunDir         string
	RepoRoot       string
	YamrSourcesDir string
	ActionFile     string
	Defaults       *yaml.Node
}

type Candidate struct {
	ActionFile string
	ActionDir  string
	Config     *yaml.Node
}

type IgnoredActionFile struct {
	ActionFile  string
	TriggeredBy string
}

type Result struct {
	Candidates         []Candidate
	IgnoredActionFiles []IgnoredActionFile
}

func Find(
	options Options,
) (Result, error) {
	if options.RunDir == "" {
		return Result{}, fmt.Errorf(
			"run directory must not be empty",
		)
	}

	runDir, err := canonicalPath(
		options.RunDir,
	)
	if err != nil {
		return Result{}, fmt.Errorf(
			"canonicalising run directory %q: %w",
			options.RunDir,
			err,
		)
	}

	if options.RepoRoot == "" {
		return Result{}, fmt.Errorf(
			"repository root must not be empty",
		)
	}

	repoRoot, err := canonicalPath(
		options.RepoRoot,
	)
	if err != nil {
		return Result{}, fmt.Errorf(
			"canonicalising repository root %q: %w",
			options.RepoRoot,
			err,
		)
	}

	withinRepo, err := pathWithin(
		repoRoot,
		runDir,
	)
	if err != nil {
		return Result{}, fmt.Errorf(
			"checking run directory against repository root: %w",
			err,
		)
	}

	if !withinRepo {
		return Result{}, fmt.Errorf(
			"run directory %q is outside repository root %q",
			runDir,
			repoRoot,
		)
	}

	if options.ActionFile == "" {
		return Result{}, fmt.Errorf(
			"action file must not be empty",
		)
	}

	defaults := options.Defaults

	if defaults == nil {
		defaults = config.EmptyDocument()
	} else {
		defaults, err = config.RunnerConfig(
			defaults,
		)
		if err != nil {
			return Result{}, fmt.Errorf(
				"extract yamr-runner defaults: %w",
				err,
			)
		}
	}

	//
	// Defaults participate in immediate token resolution just
	// like action-file configuration.
	//
	// There is deliberately no ThisDir here. The semantics of
	// $this_dir$ in the defaults file have not been defined.
	//
	if err := tokens.ResolveDocument(
		defaults,
		tokens.Context{
			RepoRoot:       repoRoot,
			RunDir:         runDir,
			YamrSourcesDir: options.YamrSourcesDir,
		},
		tokens.Immediate,
	); err != nil {
		return Result{}, fmt.Errorf(
			"resolve immediate tokens in yamr-runner defaults: %w",
			err,
		)
	}

	state := finder{
		options: Options{
			RunDir:         runDir,
			RepoRoot:       repoRoot,
			YamrSourcesDir: options.YamrSourcesDir,
			ActionFile:     options.ActionFile,
			Defaults:       defaults,
		},
	}

	//
	// Configuration inheritance starts at the repository root,
	// not the run directory.
	//
	// Action files above RunDir contribute configuration but are
	// never actionable for this invocation. Their trigger values
	// are therefore validated but do not terminate discovery.
	//
	inherited, err := state.inheritedConfig(
		defaults,
	)
	if err != nil {
		return Result{}, fmt.Errorf(
			"building inherited configuration: %w",
			err,
		)
	}

	//
	// RunDir is the actionable discovery boundary. From this
	// point down, trigger-yamr-runner has its normal semantics.
	//
	result, err := state.walk(
		runDir,
		inherited,
	)
	if err != nil {
		return Result{}, err
	}

	sort.Slice(
		result.candidates,
		func(i, j int) bool {
			return result.candidates[i].ActionFile <
				result.candidates[j].ActionFile
		},
	)

	sort.Slice(
		result.ignoredActionFiles,
		func(i, j int) bool {
			left := result.ignoredActionFiles[i]
			right := result.ignoredActionFiles[j]

			if left.TriggeredBy != right.TriggeredBy {
				return left.TriggeredBy <
					right.TriggeredBy
			}

			return left.ActionFile <
				right.ActionFile
		},
	)

	return Result{
		Candidates:         result.candidates,
		IgnoredActionFiles: result.ignoredActionFiles,
	}, nil
}

type finder struct {
	options Options
}

type walkResult struct {
	candidates         []Candidate
	ignoredActionFiles []IgnoredActionFile
}

// inheritedConfig builds the configuration inherited by RunDir.
//
// It processes action files from RepoRoot through the parent of
// RunDir, in that order.
//
// Trigger values in these files are validated but deliberately
// ignored: directories above RunDir are outside the actionable
// discovery boundary for this invocation.
func (f *finder) inheritedConfig(
	defaults *yaml.Node,
) (*yaml.Node, error) {
	dirs, err := ancestorDirs(
		f.options.RepoRoot,
		f.options.RunDir,
	)
	if err != nil {
		return nil, err
	}

	merged := defaults

	for _, dir := range dirs {
		mergedConfig, _, _, err := f.mergeActionFile(
			dir,
			merged,
		)
		if err != nil {
			return nil, err
		}

		merged = mergedConfig
	}

	return merged, nil
}

// mergeActionFile merges the action file in dir, if one exists,
// over inherited.
//
// The returned trigger value describes the action file itself.
// The caller decides whether that trigger is actionable.
//
// This distinction allows ancestor action files above RunDir to
// contribute inherited configuration without becoming candidates
// or terminating discovery.
func (f *finder) mergeActionFile(
	dir string,
	inherited *yaml.Node,
) (
	merged *yaml.Node,
	triggered bool,
	exists bool,
	err error,
) {
	actionFile := filepath.Join(
		dir,
		f.options.ActionFile,
	)

	info, err := os.Stat(
		actionFile,
	)

	switch {
	case err == nil:
		if info.IsDir() {
			return nil, false, false, fmt.Errorf(
				"action file %q is a directory",
				actionFile,
			)
		}

	case os.IsNotExist(err):
		return inherited, false, false, nil

	default:
		return nil, false, false, fmt.Errorf(
			"checking action file %q: %w",
			actionFile,
			err,
		)
	}

	document, err := config.Load(
		actionFile,
	)
	if err != nil {
		return nil, false, true, fmt.Errorf(
			"load action file %q: %w",
			actionFile,
			err,
		)
	}

	triggered, err = triggerYamrRunner(
		document,
	)
	if err != nil {
		return nil, false, true, fmt.Errorf(
			"read trigger from action file %q: %w",
			actionFile,
			err,
		)
	}

	runnerConfig, err := config.RunnerConfig(
		document,
	)
	if err != nil {
		return nil, false, true, fmt.Errorf(
			"extract yamr-runner from action file %q: %w",
			actionFile,
			err,
		)
	}

	err = tokens.ResolveDocument(
		runnerConfig,
		tokens.Context{
			ThisDir:        dir,
			RepoRoot:       f.options.RepoRoot,
			RunDir:         f.options.RunDir,
			YamrSourcesDir: f.options.YamrSourcesDir,
		},
		tokens.Immediate,
	)
	if err != nil {
		return nil, false, true, fmt.Errorf(
			"resolving immediate tokens in action file %q: %w",
			actionFile,
			err,
		)
	}

	merged, err = config.Merge(
		inherited,
		runnerConfig,
	)
	if err != nil {
		return nil, false, true, fmt.Errorf(
			"merging action file %q: %w",
			actionFile,
			err,
		)
	}

	return merged, triggered, true, nil
}

func (f *finder) walk(
	dir string,
	inherited *yaml.Node,
) (walkResult, error) {
	merged, triggered, _, err := f.mergeActionFile(
		dir,
		inherited,
	)
	if err != nil {
		return walkResult{}, err
	}

	actionFile := filepath.Join(
		dir,
		f.options.ActionFile,
	)

	//
	// A trigger terminates normal configuration discovery on
	// this filesystem branch.
	//
	// Configuration below this point cannot contribute to this
	// action and cannot create another candidate. We scan only
	// for descendant action-file names so the caller can warn
	// that they are being ignored.
	//
	if triggered {
		ignored, err := f.findDescendantActionFiles(
			dir,
			actionFile,
		)
		if err != nil {
			return walkResult{}, err
		}

		return walkResult{
			candidates: []Candidate{
				newCandidate(
					actionFile,
					dir,
					merged,
				),
			},
			ignoredActionFiles: ignored,
		}, nil
	}

	childDirs, err := directories(
		dir,
	)
	if err != nil {
		return walkResult{}, err
	}

	result := walkResult{}

	for _, childDir := range childDirs {
		child, err := f.walk(
			childDir,
			merged,
		)
		if err != nil {
			return walkResult{}, err
		}

		result.candidates = append(
			result.candidates,
			child.candidates...,
		)

		result.ignoredActionFiles = append(
			result.ignoredActionFiles,
			child.ignoredActionFiles...,
		)
	}

	return result, nil
}

func triggerYamrRunner(
	document *yaml.Node,
) (bool, error) {
	if document == nil {
		return false, fmt.Errorf(
			"document is nil",
		)
	}

	if document.Kind != yaml.DocumentNode {
		return false, fmt.Errorf(
			"expected YAML document node",
		)
	}

	if len(document.Content) != 1 {
		return false, fmt.Errorf(
			"expected YAML document to contain exactly one root node",
		)
	}

	root := document.Content[0]

	if root.Kind != yaml.MappingNode {
		return false, fmt.Errorf(
			"YAML root must be a mapping",
		)
	}

	found := false
	triggered := false

	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i]
		value := root.Content[i+1]

		if key.Value != "trigger-yamr-runner" {
			continue
		}

		if found {
			return false, fmt.Errorf(
				"duplicate top-level key %q",
				"trigger-yamr-runner",
			)
		}

		found = true

		if value.Kind != yaml.ScalarNode ||
			value.Tag != "!!bool" {
			return false, fmt.Errorf(
				"top-level trigger-yamr-runner must be a boolean",
			)
		}

		switch value.Value {
		case "true":
			triggered = true

		case "false":
			triggered = false

		default:
			return false, fmt.Errorf(
				"top-level trigger-yamr-runner must be a boolean",
			)
		}
	}

	return triggered, nil
}

// find descendant action files but exclude the action scan that made us look in the first place!
func (f *finder) findDescendantActionFiles(
	dir string,
	triggeredBy string,
) ([]IgnoredActionFile, error) {
	var result []IgnoredActionFile

	err := filepath.WalkDir(
		dir,
		func(
			path string,
			entry fs.DirEntry,
			err error,
		) error {
			if err != nil {
				return err
			}

			if path == dir {
				return nil
			}

			if entry.IsDir() {
				if entry.Name() == ".git" {
					return filepath.SkipDir
				}

				return nil
			}

			//
			// The triggering action file itself is not an
			// ignored descendant.
			//
			if path == triggeredBy {
				return nil
			}

			if entry.Name() != f.options.ActionFile {
				return nil
			}

			result = append(
				result,
				IgnoredActionFile{
					ActionFile:  path,
					TriggeredBy: triggeredBy,
				},
			)

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"scanning descendants of triggered action file %q: %w",
			triggeredBy,
			err,
		)
	}

	sort.Slice(
		result,
		func(i, j int) bool {
			return result[i].ActionFile <
				result[j].ActionFile
		},
	)

	return result, nil
}

func newCandidate(
	actionFile string,
	actionDir string,
	document *yaml.Node,
) Candidate {
	return Candidate{
		ActionFile: actionFile,
		ActionDir:  actionDir,
		Config:     document,
	}
}

func directories(
	dir string,
) ([]string, error) {
	entries, err := os.ReadDir(
		dir,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"reading directory %q: %w",
			dir,
			err,
		)
	}

	var result []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Never descend into Git's internal data.
		if entry.Name() == ".git" {
			continue
		}

		result = append(
			result,
			filepath.Join(
				dir,
				entry.Name(),
			),
		)
	}

	sort.Strings(result)

	return result, nil
}

// ancestorDirs returns the directories which participate only in
// configuration inheritance.
//
// RepoRoot is included and RunDir itself is excluded.
//
// For:
//
//	RepoRoot = /repo
//	RunDir   = /repo/a/b/c
//
// this returns:
//
//	/repo
//	/repo/a
//	/repo/a/b
func ancestorDirs(
	repoRoot string,
	runDir string,
) ([]string, error) {
	if repoRoot == runDir {
		return nil, nil
	}

	withinRepo, err := pathWithin(
		repoRoot,
		runDir,
	)
	if err != nil {
		return nil, err
	}

	if !withinRepo {
		return nil, fmt.Errorf(
			"run directory %q is outside repository root %q",
			runDir,
			repoRoot,
		)
	}

	var reversed []string

	current := filepath.Dir(
		runDir,
	)

	for {
		reversed = append(
			reversed,
			current,
		)

		if current == repoRoot {
			break
		}

		parent := filepath.Dir(
			current,
		)

		if parent == current {
			return nil, fmt.Errorf(
				"repository root %q is not an ancestor of run directory %q",
				repoRoot,
				runDir,
			)
		}

		current = parent
	}

	result := make(
		[]string,
		len(reversed),
	)

	for i := range reversed {
		result[len(reversed)-1-i] = reversed[i]
	}

	return result, nil
}

func pathWithin(
	parent string,
	child string,
) (bool, error) {
	relative, err := filepath.Rel(
		parent,
		child,
	)
	if err != nil {
		return false, err
	}

	if relative == "." {
		return true, nil
	}

	return relative != ".." &&
		!strings.HasPrefix(
			relative,
			".."+string(filepath.Separator),
		), nil
}

func canonicalPath(
	path string,
) (string, error) {
	path, err := filepath.Abs(
		path,
	)
	if err != nil {
		return "", err
	}

	path, err = filepath.EvalSymlinks(
		path,
	)
	if err != nil {
		return "", err
	}

	return filepath.Clean(path), nil
}
