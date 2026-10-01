package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
)

type PlanActionState string

const (
	PlanActionNotRun PlanActionState = "not_run"
	PlanActionRan    PlanActionState = "ran"
)

type PlanResult struct {
	Actions []PlanActionResult
}

type PlanActionResult struct {
	Index  int
	State  PlanActionState
	Result ActionResult
	Error  error
}

type PlanResultSummary struct {
	Total     int
	Ran       int
	Succeeded int
	Failed    int
	Cancelled int
	NotRun    int
}

func (r PlanResult) Failed() bool {
	for _, action := range r.Actions {
		if action.Error != nil {
			return true
		}
	}

	return false
}

func (r PlanResult) Summary() PlanResultSummary {
	summary := PlanResultSummary{
		Total: len(r.Actions),
	}

	for _, action := range r.Actions {
		if action.State == PlanActionNotRun {
			summary.NotRun++
			continue
		}

		summary.Ran++

		if errors.Is(
			action.Error,
			context.Canceled,
		) {
			summary.Cancelled++
			summary.Failed++
			continue
		}

		if action.Error != nil ||
			action.Result.Failed() {
			summary.Failed++
			continue
		}

		summary.Succeeded++
	}

	return summary
}

// RunPlan executes all planned actions with bounded concurrency.
//
// An action occupies one worker for its entire lifecycle:
//
//	pre_run -> run -> on_fail
//
// Action failures do not cancel other actions. Every planned action is
// attempted unless the supplied context is cancelled.
func (e *Execution) RunPlan(
	ctx context.Context,
	plan Plan,
	maxWorkers int,
	stdout io.Writer,
	stderr io.Writer,
) (PlanResult, error) {
	if ctx == nil {
		return PlanResult{}, fmt.Errorf(
			"context must not be nil",
		)
	}

	if maxWorkers <= 0 {
		return PlanResult{}, fmt.Errorf(
			"max workers must be greater than zero",
		)
	}

	result := PlanResult{
		Actions: make(
			[]PlanActionResult,
			len(plan.Actions),
		),
	}

	// Populate every result before dispatch starts. This means the
	// returned PlanResult always represents the complete plan, including
	// actions which were never run because the context was cancelled.
	for index, planned := range plan.Actions {
		result.Actions[index] = PlanActionResult{
			Index: index,
			State: PlanActionNotRun,
			Result: ActionResult{
				Action: planned,
			},
		}
	}

	if len(plan.Actions) == 0 {
		return result, nil
	}

	workerCount := min(
		maxWorkers,
		len(plan.Actions),
	)

	jobs := make(chan int)

	var workers sync.WaitGroup

	for range workerCount {
		workers.Add(1)

		go func() {
			defer workers.Done()

			for index := range jobs {
				// check we haven't been cancelled, or we get a race condition
				if ctx.Err() != nil {
					continue
				}

				planned := plan.Actions[index]

				actionResult, err := e.RunAction(
					ctx,
					planned,
					stdout,
					stderr,
				)

				result.Actions[index] = PlanActionResult{
					Index:  index,
					State:  PlanActionRan,
					Result: actionResult,
					Error:  err,
				}
			}
		}()
	}

dispatch:
	for index := range plan.Actions {
		// check for cancellation here to minimise race condition
		if ctx.Err() != nil {
			break dispatch
		}
		select {
		case jobs <- index:
		case <-ctx.Done():
			break dispatch
		}
	}

	close(jobs)
	workers.Wait()

	if err := ctx.Err(); err != nil {
		return result, err
	}

	return result, nil
}
