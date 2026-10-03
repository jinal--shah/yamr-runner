package runner

import (
	"fmt"
	"os"
	"path/filepath"

	"jinal--shah/yamr-runner/internal/action"
	"jinal--shah/yamr-runner/internal/docker"
)

type Plan struct {
	Actions []PlannedAction
}

type PlannedAction struct {
	Action *action.Action
	PreRun []PreRunOperation
	Docker docker.Command
	OnFail *PlannedOnFail
}

func (p Plan) Images() []string {
	seen := make(
		map[string]struct{},
	)

	var images []string

	add := func(
		image string,
	) {
		if _, exists := seen[image]; exists {
			return
		}

		seen[image] = struct{}{}

		images = append(
			images,
			image,
		)
	}

	for _, plannedAction := range p.Actions {
		add(
			plannedAction.Docker.Image,
		)

		if plannedAction.OnFail != nil {
			add(
				plannedAction.OnFail.Docker.Image,
			)
		}
	}

	return images
}

func BuildPlan(
	actions []*action.Action,
) (Plan, error) {
	plan := Plan{
		Actions: make(
			[]PlannedAction,
			0,
			len(actions),
		),
	}

	for i, a := range actions {
		if a == nil {
			return Plan{}, fmt.Errorf(
				"action %d is nil",
				i,
			)
		}

		preRun, err := CompilePreRun(a)
		if err != nil {
			return Plan{}, fmt.Errorf(
				"compile pre_run for action %q: %w",
				a.ActionFile,
				err,
			)
		}

		dockerCommand, err := docker.Compile(a)
		if err != nil {
			return Plan{}, fmt.Errorf(
				"compile docker configuration "+
					"for action %q: %w",
				a.ActionFile,
				err,
			)
		}

		onFail, err := CompileOnFail(a)
		if err != nil {
			return Plan{}, fmt.Errorf(
				"compile on_fail for action %q: %w",
				a.ActionFile,
				err,
			)
		}

		plan.Actions = append(
			plan.Actions,
			PlannedAction{
				Action: a,
				PreRun: preRun,
				Docker: dockerCommand,
				OnFail: onFail,
			},
		)
	}

	if err := ValidatePlan(plan); err != nil {
		return Plan{}, err
	}

	return plan, nil
}

func ValidatePlan(
	plan Plan,
) error {
	if err := validateMoveToTmpPaths(plan); err != nil {
		return fmt.Errorf(
			"validate execution plan: %w",
			err,
		)
	}

	if err := validateMkdirPaths(plan); err != nil {
		return fmt.Errorf(
			"validate execution plan: %w",
			err,
		)
	}

	return nil
}

func validateMoveToTmpPaths(
	plan Plan,
) error {
	type occurrence struct {
		ActionFile string
		SourcePath string
	}

	seen := make(map[string]occurrence)

	for actionIndex, planned := range plan.Actions {
		if planned.Action == nil {
			return fmt.Errorf(
				"planned action %d has nil action",
				actionIndex,
			)
		}

		for operationIndex, operation := range planned.PreRun {

			moveToTmp, ok := operation.(MoveToTmpOperation)
			if !ok {
				continue
			}

			sourcePath := moveToTmp.Path

			canonical, err := canonicalPath(
				sourcePath,
			)
			if err != nil {
				return fmt.Errorf(
					"action %q pre_run operation %d: "+
						"canonicalize mv_to_tmp path "+
						"%q: %w",
					planned.Action.ActionFile,
					operationIndex,
					sourcePath,
					err,
				)
			}

			if previous, exists :=
				seen[canonical]; exists {

				return fmt.Errorf(
					"duplicate mv_to_tmp destination: "+
						"action %q path %q and "+
						"action %q path %q resolve "+
						"to the same canonical path %q",
					previous.ActionFile,
					previous.SourcePath,
					planned.Action.ActionFile,
					sourcePath,
					canonical,
				)
			}

			seen[canonical] = occurrence{
				ActionFile: planned.Action.ActionFile,
				SourcePath: sourcePath,
			}
		}
	}

	return nil
}

func validateMkdirPaths(
	plan Plan,
) error {
	type occurrence struct {
		ActionFile string
		Path       string
		Mode       os.FileMode
		UID        int
		GID        int
	}

	seen := make(map[string]occurrence)

	for actionIndex, planned := range plan.Actions {
		if planned.Action == nil {
			return fmt.Errorf(
				"planned action %d has nil action",
				actionIndex,
			)
		}

		for operationIndex, operation := range planned.PreRun {

			mkdir, ok := operation.(MkdirOperation)
			if !ok {
				continue
			}

			canonical, err := canonicalPath(
				mkdir.Path,
			)
			if err != nil {
				return fmt.Errorf(
					"action %q pre_run operation %d: "+
						"canonicalize mkdir path %q: %w",
					planned.Action.ActionFile,
					operationIndex,
					mkdir.Path,
					err,
				)
			}

			previous, exists := seen[canonical]
			if !exists {
				seen[canonical] = occurrence{
					ActionFile: planned.Action.ActionFile,
					Path:       mkdir.Path,
					Mode:       mkdir.Mode,
					UID:        mkdir.UID,
					GID:        mkdir.GID,
				}

				continue
			}

			if previous.Mode == mkdir.Mode &&
				previous.UID == mkdir.UID &&
				previous.GID == mkdir.GID {

				continue
			}

			return fmt.Errorf(
				"conflicting mkdir configuration for "+
					"canonical path %q: "+
					"action %q path %q requests "+
					"mode %04o ownership %d:%d, "+
					"while action %q path %q requests "+
					"mode %04o ownership %d:%d",
				canonical,
				previous.ActionFile,
				previous.Path,
				previous.Mode.Perm(),
				previous.UID,
				previous.GID,
				planned.Action.ActionFile,
				mkdir.Path,
				mkdir.Mode.Perm(),
				mkdir.UID,
				mkdir.GID,
			)
		}
	}

	return nil
}

// canonicalPath returns the canonical location of path even when the
// complete path does not yet exist.
//
// It walks upward until it finds an existing ancestor, resolves all
// symlinks in that ancestor, then appends the non-existent path
// components again.
//
// This is safe for plan validation because yamr-runner assumes that
// symlinks are not created or changed during execution.
func canonicalPath(
	path string,
) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf(
			"make path absolute: %w",
			err,
		)
	}

	absolute = filepath.Clean(absolute)

	current := absolute

	var missing []string

	for {
		resolved, err := filepath.EvalSymlinks(
			current,
		)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(
					resolved,
					missing[i],
				)
			}

			resolved, err = filepath.Abs(resolved)
			if err != nil {
				return "", fmt.Errorf(
					"make resolved path absolute: %w",
					err,
				)
			}

			return filepath.Clean(
				resolved,
			), nil
		}

		if !os.IsNotExist(err) {
			return "", fmt.Errorf(
				"resolve symlinks in %q: %w",
				current,
				err,
			)
		}

		parent := filepath.Dir(current)

		if parent == current {
			return "", fmt.Errorf(
				"could not find an existing ancestor "+
					"of %q",
				path,
			)
		}

		missing = append(
			missing,
			filepath.Base(current),
		)

		current = parent
	}
}
