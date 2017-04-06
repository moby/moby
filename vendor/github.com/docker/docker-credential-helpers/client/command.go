package client

import (
	"io"
	"os/exec"
)

// Program is an interface to execute external programs.
type Program interface {
	Output() ([]byte, error)
	Input(in io.Reader)
}

// ProgramFunc is a type of function that initializes programs based on arguments.
type ProgramFunc func(args ...string) Program

// NewShellProgramFunc creates programs that are executed in a Shell.
func NewShellProgramFunc(name string) ProgramFunc {
	return func(args ...string) Program {
		return &Shell{cmd: exec.Command(name, args...)}
	}
}

// Shell invokes shell commands to talk with a remote credentials helper.
type Shell struct {
	cmd *exec.Cmd
}

// Output returns responses from the remote credentials helper.
func (s *Shell) Output() ([]byte, error) {
	return s.cmd.Output()
}

// Input sets the input to send to a remote credentials helper.
func (s *Shell) Input(in io.Reader) {
	s.cmd.Stdin = in
}
