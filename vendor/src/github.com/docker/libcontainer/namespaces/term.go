package namespaces

import (
	"io"
	"os"
	"os/exec"
)

type Terminal interface {
	io.Closer
	SetMaster(*os.File)
	Attach(*exec.Cmd) error
	Resize(h, w int) error
}

func NewTerminal(stdin io.Reader, stdout, stderr io.Writer, tty bool) Terminal {
	if tty {
		return &TtyTerminal{
			stdin:  stdin,
			stdout: stdout,
			stderr: stderr,
		}
	}
	return &StdTerminal{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}
}
