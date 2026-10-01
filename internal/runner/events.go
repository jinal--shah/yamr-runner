package runner

import "jinal--shah/yamr-run/internal/docker"

type EventSink interface {
	MvToTmp(MvToTmpEvent)
	Mkdir(MkdirEvent)
	PreRunFailed(PreRunFailedEvent)

	ActionStarted(ActionStartedEvent)
	ActionFinished(ActionFinishedEvent)

	OnFailStarted(OnFailStartedEvent)
	OnFailFinished(OnFailFinishedEvent)
	OnFailPreparationFailed(OnFailPreparationFailedEvent)
}

type MvToTmpEvent struct {
	ActionFile  string
	Source      string
	Destination string
}

type MkdirEvent struct {
	ActionFile string
	Path       string
}

type PreRunFailedEvent struct {
	ActionFile string
	Operation  PreRunOperationType
	Err        error
}

type ActionStartedEvent struct {
	ActionFile string
	Image      string
}

type ActionFinishedEvent struct {
	ActionFile string
	Image      string
	Result     docker.Result
	Err        error

	StdoutPath string
	StderrPath string
}

type OnFailStartedEvent struct {
	ActionFile string
	Image      string

	StdoutPath string
	StderrPath string
}

type OnFailPreparationFailedEvent struct {
	ActionFile string
	Err        error
}

type OnFailFinishedEvent struct {
	ActionFile string
	Image      string
	Result     docker.Result
	Err        error

	StdoutPath string
	StderrPath string
}

func (e *Execution) emitMvToTmp(
	event MvToTmpEvent,
) {
	if e.events != nil {
		e.events.MvToTmp(event)
	}
}

func (e *Execution) emitMkdir(
	event MkdirEvent,
) {
	if e.events != nil {
		e.events.Mkdir(event)
	}
}

func (e *Execution) emitPreRunFailed(
	event PreRunFailedEvent,
) {
	if e.events != nil {
		e.events.PreRunFailed(event)
	}
}

func (e *Execution) emitActionStarted(
	event ActionStartedEvent,
) {
	if e.events != nil {
		e.events.ActionStarted(event)
	}
}

func (e *Execution) emitActionFinished(
	event ActionFinishedEvent,
) {
	if e.events != nil {
		e.events.ActionFinished(event)
	}
}

func (e *Execution) emitOnFailPreparationFailed(
	event OnFailPreparationFailedEvent,
) {
	if e.events != nil {
		e.events.OnFailPreparationFailed(event)
	}
}

func (e *Execution) emitOnFailStarted(
	event OnFailStartedEvent,
) {
	if e.events != nil {
		e.events.OnFailStarted(event)
	}
}

func (e *Execution) emitOnFailFinished(
	event OnFailFinishedEvent,
) {
	if e.events != nil {
		e.events.OnFailFinished(event)
	}
}

