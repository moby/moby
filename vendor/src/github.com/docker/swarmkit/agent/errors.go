package agent

import (
	"errors"
	"fmt"
)

var (
	// ErrAgentClosed is returned by agent methods after the agent has been
	// fully closed.
	ErrAgentClosed = errors.New("agent: closed")

	errNodeNotRegistered = fmt.Errorf("node not registered")
)
