package runner

import (
// "context"
// "io"
// "path/filepath"
// "testing"
// "jinal--shah/yamr-run/internal/action"
// "jinal--shah/yamr-run/internal/docker"
)

type recordingEventSink struct {
	mvToTmp                 []MvToTmpEvent
	mkdir                   []MkdirEvent
	preRunFailed            []PreRunFailedEvent
	actionStarted           []ActionStartedEvent
	actionFinished          []ActionFinishedEvent
	onFailPreparationFailed []OnFailPreparationFailedEvent
	onFailStarted           []OnFailStartedEvent
	onFailFinished          []OnFailFinishedEvent
	events                  []recordedEvent
}

type recordedEvent struct {
	Kind  string
	Value any
}

func (r *recordingEventSink) MvToTmp(
	event MvToTmpEvent,
) {
	r.mvToTmp = append(
		r.mvToTmp,
		event,
	)

	r.events = append(
		r.events,
		recordedEvent{
			Kind:  "mv_to_tmp",
			Value: event,
		},
	)
}

func (r *recordingEventSink) Mkdir(
	event MkdirEvent,
) {
	r.mkdir = append(
		r.mkdir,
		event,
	)

	r.events = append(
		r.events,
		recordedEvent{
			Kind:  "mkdir",
			Value: event,
		},
	)
}

func (r *recordingEventSink) PreRunFailed(
	event PreRunFailedEvent,
) {
	r.preRunFailed = append(
		r.preRunFailed,
		event,
	)

	r.events = append(
		r.events,
		recordedEvent{
			Kind:  "pre_run_failed",
			Value: event,
		},
	)
}

func (r *recordingEventSink) ActionStarted(
	event ActionStartedEvent,
) {
	r.actionStarted = append(
		r.actionStarted,
		event,
	)

	r.events = append(
		r.events,
		recordedEvent{
			Kind:  "action_started",
			Value: event,
		},
	)
}

func (r *recordingEventSink) ActionFinished(
	event ActionFinishedEvent,
) {
	r.actionFinished = append(
		r.actionFinished,
		event,
	)

	r.events = append(
		r.events,
		recordedEvent{
			Kind:  "action_finished",
			Value: event,
		},
	)
}

func (r *recordingEventSink) OnFailPreparationFailed(
	event OnFailPreparationFailedEvent,
) {
	r.onFailPreparationFailed = append(
		r.onFailPreparationFailed,
		event,
	)

	r.events = append(
		r.events,
		recordedEvent{
			Kind:  "on_fail_preparation_failed",
			Value: event,
		},
	)
}

func (r *recordingEventSink) OnFailStarted(
	event OnFailStartedEvent,
) {
	r.onFailStarted = append(
		r.onFailStarted,
		event,
	)

	r.events = append(
		r.events,
		recordedEvent{
			Kind:  "on_fail_action_started",
			Value: event,
		},
	)
}

func (r *recordingEventSink) OnFailFinished(
	event OnFailFinishedEvent,
) {
	r.onFailFinished = append(
		r.onFailFinished,
		event,
	)

	r.events = append(
		r.events,
		recordedEvent{
			Kind:  "on_fail_action_finished",
			Value: event,
		},
	)
}
