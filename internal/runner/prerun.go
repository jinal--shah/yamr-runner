package runner

/*
To add a new PreRunOperationType, just satisfy the interface.

Add:
	const PreRunOperationRemove PreRunOperationType = "remove"

	type RemoveOperation struct {
		Path string
	}

	func (RemoveOperation) Type() PreRunOperationType
	func (RemoveOperation) run(*Execution, string) error
	func parseRemove(*yaml.Node) (RemoveOperation, error)

and then add it to the case statement in parsePreRunOperation
*/

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"jinal--shah/yamr-run/internal/action"

	"gopkg.in/yaml.v3"
)

const defaultMkdirMode os.FileMode = 0o755

type PreRunOperationType string

const (
	PreRunOperationMvToTmp PreRunOperationType = "mv_to_tmp"
	PreRunOperationMkdir   PreRunOperationType = "mkdir"
)

type PreRunOperation interface {
	Type() PreRunOperationType

	run(
		execution *Execution,
		actionFile string,
	) error
}

type MoveToTmpOperation struct {
	Path string
}

func (o MoveToTmpOperation) Type() PreRunOperationType {
	return PreRunOperationMvToTmp
}

type MkdirOperation struct {
	Path string
	Mode os.FileMode
	UID  int
	GID  int
}

func (o MkdirOperation) Type() PreRunOperationType {
	return PreRunOperationMkdir
}

// TempRoot returns the temporary directory shared by all mv_to_tmp
// operations belonging to this CLI execution.
//
// The directory is created lazily. An execution which never moves
// anything therefore creates no temporary directory.
func (e *Execution) TempRoot() (string, error) {
	e.tempOnce.Do(func() {
		now := time.Now()

		prefix := fmt.Sprintf(
			"yamr-run-%s_%03d-",
			now.Format("2006-01-02_15_04_05"),
			now.Nanosecond()/1_000_000,
		)

		e.tempRoot, e.tempErr = os.MkdirTemp(
			"",
			prefix,
		)
	})

	return e.tempRoot, e.tempErr
}

// CompilePreRun extracts and validates pre_run operations from an
// already compiled action.
//
// All tokens in the action are expected to have been resolved before
// this function is called.
func CompilePreRun(
	a *action.Action,
) ([]PreRunOperation, error) {
	if a == nil {
		return nil, fmt.Errorf("action must not be nil")
	}

	if a.Config == nil {
		return nil, fmt.Errorf(
			"action %q has nil config",
			a.ActionFile,
		)
	}

	preRun, err := preRunNode(a.Config)
	if err != nil {
		return nil, actionError(a, err)
	}

	if preRun == nil || preRun.Tag == "!!null" {
		return nil, nil
	}

	if preRun.Kind != yaml.SequenceNode {
		return nil, actionError(
			a,
			fmt.Errorf(
				"yamr-runner.action.pre_run must be a sequence",
			),
		)
	}

	operations := make(
		[]PreRunOperation,
		0,
		len(preRun.Content),
	)

	for i, node := range preRun.Content {
		operation, err := parsePreRunOperation(node)
		if err != nil {
			return nil, actionError(
				a,
				fmt.Errorf(
					"yamr-runner.action.pre_run[%d]: %w",
					i,
					err,
				),
			)
		}

		operations = append(
			operations,
			operation,
		)
	}

	return operations, nil
}

func parsePreRunOperation(
	node *yaml.Node,
) (PreRunOperation, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"must be a mapping",
		)
	}

	// A mapping contains alternating key/value nodes. Exactly one
	// operation therefore means exactly two Content entries.
	if len(node.Content) != 2 {
		return nil, fmt.Errorf(
			"must contain exactly one operation",
		)
	}

	name := node.Content[0]
	config := node.Content[1]

	if name.Kind != yaml.ScalarNode ||
		name.Tag != "!!str" {
		return nil, fmt.Errorf(
			"operation name must be a string",
		)
	}

	if config.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"%s must be a mapping",
			name.Value,
		)
	}

	switch PreRunOperationType(name.Value) {
	case PreRunOperationMvToTmp:
		operation, err := parseMoveToTmp(
			config,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"%s: %w",
				PreRunOperationMvToTmp,
				err,
			)
		}

		return operation, nil

	case PreRunOperationMkdir:
		operation, err := parseMkdir(
			config,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"%s: %w",
				PreRunOperationMkdir,
				err,
			)
		}

		return operation, nil

	default:
		return nil, fmt.Errorf(
			"unsupported operation %q",
			name.Value,
		)
	}
}

func parseMoveToTmp(
	config *yaml.Node,
) (MoveToTmpOperation, error) {
	if err := validateMappingKeys(
		config,
		"path",
	); err != nil {
		return MoveToTmpOperation{}, err
	}

	path, err := requiredPath(config, "path")
	if err != nil {
		return MoveToTmpOperation{}, err
	}

	return MoveToTmpOperation{
		Path: path,
	}, nil
}

func parseMkdir(
	config *yaml.Node,
) (MkdirOperation, error) {
	if err := validateMappingKeys(
		config,
		"path",
		"chmod",
		"chown",
	); err != nil {
		return MkdirOperation{}, err
	}

	path, err := requiredPath(config, "path")
	if err != nil {
		return MkdirOperation{}, err
	}

	mode, err := parseMode(
		mappingValue(config, "chmod"),
	)
	if err != nil {
		return MkdirOperation{}, err
	}

	uid := os.Geteuid()
	gid := os.Getegid()

	chown := mappingValue(config, "chown")
	if chown != nil && chown.Tag != "!!null" {
		if chown.Kind != yaml.ScalarNode {
			return MkdirOperation{}, fmt.Errorf(
				"chown must be a scalar",
			)
		}

		uid, gid, err = parseOwnership(
			chown.Value,
		)
		if err != nil {
			return MkdirOperation{}, err
		}
	}

	return MkdirOperation{
		Path: path,
		Mode: mode,
		UID:  uid,
		GID:  gid,
	}, nil
}

func requiredPath(
	mapping *yaml.Node,
	key string,
) (string, error) {
	node := mappingValue(mapping, key)
	if node == nil {
		return "", fmt.Errorf(
			"%s is required",
			key,
		)
	}

	if node.Kind != yaml.ScalarNode ||
		node.Tag == "!!null" {
		return "", fmt.Errorf(
			"%s must be a string",
			key,
		)
	}

	path := strings.TrimSpace(node.Value)
	if path == "" {
		return "", fmt.Errorf(
			"%s must not be empty",
			key,
		)
	}

	if !filepath.IsAbs(path) {
		return "", fmt.Errorf(
			"%s %q must be absolute",
			key,
			path,
		)
	}

	return filepath.Clean(path), nil
}

// parseMode interprets chmod values as octal based on their lexical
// representation.
//
// This deliberately does not depend on whether the YAML parser tagged
// the scalar as a string or integer. Consequently all of:
//
//	chmod: "0755"
//	chmod: 0755
//	chmod: 755
//	chmod: 0o755
//
// mean mode 0755.
func parseMode(
	node *yaml.Node,
) (os.FileMode, error) {
	if node == nil || node.Tag == "!!null" {
		return defaultMkdirMode, nil
	}

	if node.Kind != yaml.ScalarNode {
		return 0, fmt.Errorf(
			"chmod must be a scalar",
		)
	}

	value := strings.TrimSpace(node.Value)
	if value == "" {
		return 0, fmt.Errorf(
			"chmod must not be empty",
		)
	}

	value = strings.TrimPrefix(value, "0o")
	value = strings.TrimPrefix(value, "0O")

	// A leading zero is optional because chmod values are always
	// interpreted as octal by this configuration field.
	value = strings.TrimLeft(value, "0")
	if value == "" {
		value = "0"
	}

	parsed, err := strconv.ParseUint(
		value,
		8,
		32,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"invalid chmod %q: expected octal permissions",
			node.Value,
		)
	}

	if parsed > 0o7777 {
		return 0, fmt.Errorf(
			"invalid chmod %q: permissions exceed 07777",
			node.Value,
		)
	}

	return os.FileMode(parsed), nil
}

func parseOwnership(
	value string,
) (int, int, error) {
	value = strings.TrimSpace(value)

	uidValue, gidValue, found := strings.Cut(
		value,
		":",
	)
	if !found {
		return 0, 0, fmt.Errorf(
			"chown must have format uid:gid",
		)
	}

	uidValue = strings.TrimSpace(uidValue)
	gidValue = strings.TrimSpace(gidValue)

	uid, err := strconv.Atoi(uidValue)
	if err != nil || uid < 0 {
		return 0, 0, fmt.Errorf(
			"invalid uid %q",
			uidValue,
		)
	}

	gid, err := strconv.Atoi(gidValue)
	if err != nil || gid < 0 {
		return 0, 0, fmt.Errorf(
			"invalid gid %q",
			gidValue,
		)
	}

	return uid, gid, nil
}

func validateMappingKeys(
	mapping *yaml.Node,
	allowed ...string,
) error {
	allowedKeys := make(
		map[string]struct{},
		len(allowed),
	)

	for _, key := range allowed {
		allowedKeys[key] = struct{}{}
	}

	seen := make(map[string]struct{})

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i]

		if key.Kind != yaml.ScalarNode ||
			key.Tag != "!!str" {
			return fmt.Errorf(
				"configuration keys must be strings",
			)
		}

		if _, ok := allowedKeys[key.Value]; !ok {
			return fmt.Errorf(
				"unknown option %q",
				key.Value,
			)
		}

		if _, ok := seen[key.Value]; ok {
			return fmt.Errorf(
				"duplicate option %q",
				key.Value,
			)
		}

		seen[key.Value] = struct{}{}
	}

	return nil
}

// RunPreRun executes operations in their configured order.
//
// Execution is shared across all actions in one CLI invocation so that
// all mv_to_tmp operations use the same temporary root.
func (e *Execution) RunPreRun(
	actionFile string,
	operations []PreRunOperation,
) error {
	for i, operation := range operations {
		if operation == nil {
			return fmt.Errorf(
				"pre_run operation %d is nil",
				i,
			)
		}

		if err := operation.run(
			e,
			actionFile,
		); err != nil {
			return fmt.Errorf(
				"pre_run operation %d (%s): %w",
				i,
				operation.Type(),
				err,
			)
		}
	}

	return nil
}

func (o MkdirOperation) run(
	execution *Execution,
	actionFile string,
) error {
	_, err := os.Stat(
		o.Path,
	)

	switch {
	case err == nil:
		// The requested path already exists.

	case os.IsNotExist(err):
		// The operation will create the requested path.

	default:
		execution.emitPreRunFailed(
			PreRunFailedEvent{
				ActionFile: actionFile,
				Operation:  o.Type(),
				Err: fmt.Errorf(
					"inspect directory %q: %w",
					o.Path,
					err,
				),
			},
		)

		return fmt.Errorf(
			"inspect directory %q: %w",
			o.Path,
			err,
		)
	}

	existed := err == nil

	if err := o.mkdir(); err != nil {
		execution.emitPreRunFailed(
			PreRunFailedEvent{
				ActionFile: actionFile,
				Operation:  o.Type(),
				Err:        err,
			},
		)

		return err
	}

	if !existed {
		execution.emitMkdir(
			MkdirEvent{
				ActionFile: actionFile,
				Path:       o.Path,
			},
		)
	}

	return nil
}

func (o MkdirOperation) mkdir() error {
	if err := os.MkdirAll(
		o.Path,
		o.Mode,
	); err != nil {
		return fmt.Errorf(
			"create directory %q: %w",
			o.Path,
			err,
		)
	}

	// MkdirAll is affected by umask and does not change the mode
	// of an existing directory. Apply the configured mode explicitly
	// to the requested directory.
	if err := os.Chmod(
		o.Path,
		o.Mode,
	); err != nil {
		return fmt.Errorf(
			"chmod %q %04o: %w",
			o.Path,
			o.Mode.Perm(),
			err,
		)
	}

	if err := os.Chown(
		o.Path,
		o.UID,
		o.GID,
	); err != nil {
		return fmt.Errorf(
			"chown %q %d:%d: %w",
			o.Path,
			o.UID,
			o.GID,
			err,
		)
	}

	return nil
}

func (o MoveToTmpOperation) run(
	execution *Execution,
	actionFile string,
) error {
	destination, moved, err := o.moveToTmp(
		execution,
	)
	if err != nil {
		execution.emitPreRunFailed(
			PreRunFailedEvent{
				ActionFile: actionFile,
				Operation:  o.Type(),
				Err:        err,
			},
		)

		return err
	}

	if !moved {
		return nil
	}

	execution.emitMvToTmp(
		MvToTmpEvent{
			ActionFile:  actionFile,
			Source:      o.Path,
			Destination: destination,
		},
	)

	return nil
}

func (o MoveToTmpOperation) moveToTmp(
	execution *Execution,
) (string, bool, error) {
	_, err := os.Lstat(o.Path)

	switch {
	case err == nil:
		// Continue.

	case os.IsNotExist(err):
		// A missing source is deliberately a no-op.
		return "", false, nil

	default:
		return "", false, fmt.Errorf(
			"inspect source %q: %w",
			o.Path,
			err,
		)
	}

	tempRoot, err := execution.TempRoot()
	if err != nil {
		return "", false, fmt.Errorf(
			"create execution temporary directory: %w",
			err,
		)
	}

	destination, err := temporaryDestination(
		tempRoot,
		o.Path,
	)
	if err != nil {
		return "", false, err
	}

	// This is a defensive runtime check. Cross-action duplicate
	// mv_to_tmp paths will be rejected by plan validation before
	// execution begins.
	//
	// Crucially, this happens before touching the source.
	_, err = os.Lstat(destination)
	switch {
	case err == nil:
		return "", false, fmt.Errorf(
			"temporary destination %q already exists",
			destination,
		)

	case os.IsNotExist(err):
		// Continue.

	default:
		return "", false, fmt.Errorf(
			"inspect temporary destination %q: %w",
			destination,
			err,
		)
	}

	if err := os.MkdirAll(
		filepath.Dir(destination),
		0o755,
	); err != nil {
		return "", false, fmt.Errorf(
			"create temporary path hierarchy: %w",
			err,
		)
	}

	if err := movePath(
		o.Path,
		destination,
	); err != nil {
		return "", false, fmt.Errorf(
			"move %q to %q: %w",
			o.Path,
			destination,
			err,
		)
	}

	return destination, true, nil
}

// temporaryDestination reproduces the absolute source hierarchy below
// the execution's temporary root.
//
// For example:
//
//	source:   /home/user/repo/.generated
//	tempRoot: /tmp/yamr-run-...
//
// becomes:
//
//	/tmp/yamr-run-.../home/user/repo/.generated
func temporaryDestination(
	tempRoot string,
	source string,
) (string, error) {
	if !filepath.IsAbs(source) {
		return "", fmt.Errorf(
			"path %q must be absolute",
			source,
		)
	}

	clean := filepath.Clean(source)

	volume := filepath.VolumeName(clean)

	withoutVolume := strings.TrimPrefix(
		clean,
		volume,
	)

	withoutRoot := strings.TrimLeft(
		withoutVolume,
		string(filepath.Separator),
	)

	if withoutRoot == "" {
		return "", fmt.Errorf(
			"cannot move filesystem root to temporary storage",
		)
	}

	return filepath.Join(
		tempRoot,
		withoutRoot,
	), nil
}

func movePath(
	source string,
	destination string,
) error {
	// Never intentionally replace an existing destination.
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf(
			"destination %q already exists",
			destination,
		)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf(
			"inspect destination: %w",
			err,
		)
	}

	err := os.Rename(source, destination)
	if err == nil {
		return nil
	}

	// os.Rename cannot move across filesystems. The system temp
	// directory is often on the same filesystem, but that is not
	// guaranteed, so fall back to copy followed by removal.
	info, statErr := os.Lstat(source)
	if statErr != nil {
		return fmt.Errorf(
			"rename failed (%v); inspect source: %w",
			err,
			statErr,
		)
	}

	switch {
	case info.Mode().IsRegular():
		if copyErr := copyFile(
			source,
			destination,
			info.Mode(),
		); copyErr != nil {
			return fmt.Errorf(
				"rename failed (%v); "+
					"copy fallback failed: %w",
				err,
				copyErr,
			)
		}

	case info.IsDir():
		if copyErr := copyDirectory(
			source,
			destination,
		); copyErr != nil {
			return fmt.Errorf(
				"rename failed (%v); "+
					"copy fallback failed: %w",
				err,
				copyErr,
			)
		}

	default:
		return fmt.Errorf(
			"rename failed (%v) and source type "+
				"does not support copy fallback",
			err,
		)
	}

	if removeErr := os.RemoveAll(source); removeErr != nil {
		// The move has not completed successfully. Remove the copied
		// destination so that the original remains the authoritative
		// location.
		_ = os.RemoveAll(destination)

		return fmt.Errorf(
			"remove original after copy: %w",
			removeErr,
		)
	}

	return nil
}

func copyDirectory(
	source string,
	destination string,
) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	// Mkdir rather than MkdirAll ensures an unexpected destination
	// collision cannot silently merge two directory trees.
	if err := os.Mkdir(
		destination,
		info.Mode().Perm(),
	); err != nil {
		return err
	}

	success := false

	defer func() {
		if !success {
			_ = os.RemoveAll(destination)
		}
	}()

	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		sourcePath := filepath.Join(
			source,
			entry.Name(),
		)

		destinationPath := filepath.Join(
			destination,
			entry.Name(),
		)

		entryInfo, err := os.Lstat(sourcePath)
		if err != nil {
			return err
		}

		switch {
		case entryInfo.Mode().IsRegular():
			if err := copyFile(
				sourcePath,
				destinationPath,
				entryInfo.Mode(),
			); err != nil {
				return err
			}

		case entryInfo.IsDir():
			if err := copyDirectory(
				sourcePath,
				destinationPath,
			); err != nil {
				return err
			}

		default:
			return fmt.Errorf(
				"unsupported file type %q",
				sourcePath,
			)
		}
	}

	success = true

	return nil
}

func copyFile(
	source string,
	destination string,
	mode os.FileMode,
) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(
		destination,
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		mode.Perm(),
	)
	if err != nil {
		return err
	}

	success := false

	defer func() {
		_ = output.Close()

		if !success {
			_ = os.Remove(destination)
		}
	}()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}

	if err := output.Close(); err != nil {
		return err
	}

	success = true

	return nil
}

func preRunNode(
	document *yaml.Node,
) (*yaml.Node, error) {
	root, err := documentMapping(document)
	if err != nil {
		return nil, err
	}

	runner := mappingValue(root, "yamr-runner")
	if runner == nil ||
		runner.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"yamr-runner must be a mapping",
		)
	}

	actionNode := mappingValue(runner, "action")
	if actionNode == nil ||
		actionNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"yamr-runner.action must be a mapping",
		)
	}

	return mappingValue(
		actionNode,
		"pre_run",
	), nil
}

func documentMapping(
	document *yaml.Node,
) (*yaml.Node, error) {
	if document == nil {
		return nil, fmt.Errorf(
			"config is nil",
		)
	}

	if document.Kind != yaml.DocumentNode ||
		len(document.Content) != 1 {
		return nil, fmt.Errorf(
			"config must be a YAML document",
		)
	}

	root := document.Content[0]

	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf(
			"config root must be a mapping",
		)
	}

	return root, nil
}

func mappingValue(
	mapping *yaml.Node,
	key string,
) *yaml.Node {
	if mapping == nil ||
		mapping.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}

	return nil
}

func actionError(
	a *action.Action,
	err error,
) error {
	if a.ActionFile == "" {
		return err
	}

	return fmt.Errorf(
		"action %q: %w",
		a.ActionFile,
		err,
	)
}
