package execdriver

import (
	"io"
	"os/exec"
)

// StdConsole defines standard console operations for execdriver
type StdConsole struct {
	// Closers holds io.Closer references for closing at terminal close time
	Closers []io.Closer
}

// NewStdConsole returns a new StdConsole struct
func NewStdConsole(processConfig *ProcessConfig, pipes *Pipes) (*StdConsole, error) {
	std := &StdConsole{}

	if err := std.AttachPipes(&processConfig.Cmd, pipes); err != nil {
		return nil, err
	}
	return std, nil
}

// AttachPipes attaches given pipes to exec.Cmd
func (s *StdConsole) AttachPipes(command *exec.Cmd, pipes *Pipes) error {
	command.Stdout = pipes.Stdout
	command.Stderr = pipes.Stderr

	if pipes.Stdin != nil {
		stdin, err := command.StdinPipe()
		if err != nil {
			return err
		}

		go func() {
			defer stdin.Close()
			io.Copy(stdin, pipes.Stdin)
		}()
	}
	return nil
}

// Resize implements Resize method of Terminal interface
func (s *StdConsole) Resize(h, w int) error {
	// we do not need to resize a non tty
	return nil
}

// Close implements Close method of Terminal interface
func (s *StdConsole) Close() error {
	for _, c := range s.Closers {
		c.Close()
	}
	return nil
}
