package lmctfy

import (
	"github.com/dotcloud/docker/pkg/system"
	"github.com/dotcloud/docker/pkg/term"
	"github.com/dotcloud/docker/runtime/execdriver"
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

func setupTerminal(c *execdriver.Command, pipes *execdriver.Pipes) (Terminal, error) {
	if !c.Tty {
		return &stdTerm{pipes: pipes}, nil
	}
	master, console, err := system.CreateMasterAndConsole()
	if err != nil {
		return nil, err
	}
	var term = ttyTerm{pipes: pipes}
	term.SetMaster(master)
	c.Console = console
	return &term, nil
}

type stdTerm struct {
	execdriver.StdConsole
	pipes *execdriver.Pipes
}

func (t *stdTerm) Attach(cmd *exec.Cmd) error {
	return t.AttachPipes(cmd, t.pipes)
}

func (t *stdTerm) SetMaster(master *os.File) {
	// do nothing
}

func (t *stdTerm) Resize(h, w int) error {
	return nil
}

func (t *stdTerm) Close() error {
	return nil
}

type ttyTerm struct {
	execdriver.TtyConsole
	pipes   *execdriver.Pipes
	console string
}

func (t *ttyTerm) Attach(cmd *exec.Cmd) error {
	go io.Copy(t.pipes.Stdout, t.MasterPty)
	if t.pipes.Stdin != nil {
		go io.Copy(t.MasterPty, t.pipes.Stdin)
	}
	return nil
}

func (t *ttyTerm) SetMaster(master *os.File) {
	t.MasterPty = master
}

func (t *ttyTerm) Resize(h, w int) error {
	err := term.SetWinsize(t.MasterPty.Fd(), &term.Winsize{Height: uint16(h), Width: uint16(w)})
	return err
}

func (t *ttyTerm) Close() error {
	t.SlavePty.Close()
	return t.MasterPty.Close()
}
