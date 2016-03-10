package credentials

import (
	"io"
	"os/exec"
)

func shellCommandFn(storeName string) func(args ...string) command {
	name := remoteCredentialsPrefix + storeName
	return func(args ...string) command {
		return &shell{cmd: exec.Command(name, args...)}
	}
}

// shell invokes shell commands to talk with a remote credentials helper.
type shell struct {
	cmd *exec.Cmd
}

// Output returns responses from the remote credentials helper.
func (s *shell) Output() ([]byte, error) {
	return s.cmd.Output()
}

// Input sets the input to send to a remote credentials helper.
func (s *shell) Input(in io.Reader) {
	s.cmd.Stdin = in
}
