package agent

import (
	"errors"
)

var (
	// ErrClosed is returned when an operation fails because the resource is closed.
	ErrClosed = errors.New("agent: closed")

	errNodeNotRegistered = errors.New("node not registered")

	errAgentStarted    = errors.New("agent: already started")
	errAgentNotStarted = errors.New("agent: not started")

	errTaskNoContoller          = errors.New("agent: no task controller")
	errTaskNotAssigned          = errors.New("agent: task not assigned")
	errTaskStatusUpdateNoChange = errors.New("agent: no change in task status")
	errTaskUnknown              = errors.New("agent: task unknown")

	errTaskInvalid = errors.New("task: invalid")
)
