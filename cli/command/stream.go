package command

import (
	"github.com/docker/docker/pkg/term"
)

// CommonStream is an input stream used by the DockerCli to read user input
type CommonStream struct {
	fd         uintptr
	isTerminal bool
	state      *term.State
}

// FD returns the file descriptor number for this stream
func (s *CommonStream) FD() uintptr {
	return s.fd
}

// IsTerminal returns true if this stream is connected to a terminal
func (s *CommonStream) IsTerminal() bool {
	return s.isTerminal
}

// RestoreTerminal restores normal mode to the terminal
func (s *CommonStream) RestoreTerminal() {
	if s.state != nil {
		term.RestoreTerminal(s.fd, s.state)
	}
}

// SetIsTerminal sets the boolean used for isTerminal
func (s *CommonStream) SetIsTerminal(isTerminal bool) {
	s.isTerminal = isTerminal
}
