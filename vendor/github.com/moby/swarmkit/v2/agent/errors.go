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

	errTaskUnknown = errors.New("agent: task unknown")
)
