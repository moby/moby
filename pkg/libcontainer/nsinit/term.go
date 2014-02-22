package nsinit

import (
	"github.com/dotcloud/docker/pkg/term"
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
		command.Process.Kill()
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

type StdTerminal struct {
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (s *StdTerminal) SetMaster(*os.File) {
	// no need to set master on non tty
}

func (s *StdTerminal) Close() error {
	return nil
}

func (s *StdTerminal) Resize(h, w int) error {
	return nil
}

func (s *StdTerminal) Attach(command *exec.Cmd) error {
	inPipe, err := command.StdinPipe()
	if err != nil {
		return err
	}
	outPipe, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	errPipe, err := command.StderrPipe()
	if err != nil {
		return err
	}

	go func() {
		defer inPipe.Close()
		io.Copy(inPipe, s.stdin)
	}()

	go io.Copy(s.stdout, outPipe)
	go io.Copy(s.stderr, errPipe)

	return nil
}
