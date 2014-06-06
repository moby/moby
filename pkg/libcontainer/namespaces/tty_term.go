package namespaces

import (
	"io"
	"os"
	"os/exec"

	"github.com/dotcloud/docker/pkg/term"
)

type TtyTerminal struct {
	stdin          io.Reader
	stdout, stderr io.Writer
	master         *os.File
	state          *term.State
}

func (t *TtyTerminal) Resize(h, w int) error {
	return term.SetWinsize(t.master.Fd(), &term.Winsize{Height: uint16(h), Width: uint16(w)})
}

func (t *TtyTerminal) SetMaster(master *os.File) {
	t.master = master
}

func (t *TtyTerminal) Attach(command *exec.Cmd) error {
	go io.Copy(t.stdout, t.master)
	go io.Copy(t.master, t.stdin)

	state, err := t.setupWindow(t.master, os.Stdin)

	if err != nil {
		return err
	}

	t.state = state
	return err
}

// SetupWindow gets the parent window size and sets the master
// pty to the current size and set the parents mode to RAW
func (t *TtyTerminal) setupWindow(master, parent *os.File) (*term.State, error) {
	ws, err := term.GetWinsize(parent.Fd())
	if err != nil {
		return nil, err
	}
	if err := term.SetWinsize(master.Fd(), ws); err != nil {
		return nil, err
	}
	return term.SetRawTerminal(parent.Fd())
}

func (t *TtyTerminal) Close() error {
	term.RestoreTerminal(os.Stdin.Fd(), t.state)
	return t.master.Close()
}
