package execdriver

import (
	"github.com/dotcloud/docker/pkg/term"
	"github.com/kr/pty"
	"io"
	"os"
	"os/exec"
)

func SetTerminal(command *Command, pipes *Pipes) error {
	var (
		term Terminal
		err  error
	)
	if command.Tty {
		term, err = NewTtyConsole(command, pipes)
	} else {
		term, err = NewStdConsole(command, pipes)
	}
	if err != nil {
		return err
	}
	command.Terminal = term
	return nil
}

type TtyConsole struct {
	MasterPty *os.File
	SlavePty  *os.File
}

func NewTtyConsole(command *Command, pipes *Pipes) (*TtyConsole, error) {
	ptyMaster, ptySlave, err := pty.Open()
	if err != nil {
		return nil, err
	}
	tty := &TtyConsole{
		MasterPty: ptyMaster,
		SlavePty:  ptySlave,
	}
	if err := tty.AttachPipes(&command.Cmd, pipes); err != nil {
		tty.Close()
		return nil, err
	}
	command.Console = tty.SlavePty.Name()
	return tty, nil
}

func (t *TtyConsole) Master() *os.File {
	return t.MasterPty
}

func (t *TtyConsole) Resize(h, w int) error {
	return term.SetWinsize(t.MasterPty.Fd(), &term.Winsize{Height: uint16(h), Width: uint16(w)})
}

func (t *TtyConsole) AttachPipes(command *exec.Cmd, pipes *Pipes) error {
	command.Stdout = t.SlavePty
	command.Stderr = t.SlavePty

	go func() {
		if wb, ok := pipes.Stdout.(interface {
			CloseWriters() error
		}); ok {
			defer wb.CloseWriters()
		}
		io.Copy(pipes.Stdout, t.MasterPty)
	}()

	if pipes.Stdin != nil {
		command.Stdin = t.SlavePty
		command.SysProcAttr.Setctty = true

		go func() {
			defer pipes.Stdin.Close()
			io.Copy(t.MasterPty, pipes.Stdin)
		}()
	}
	return nil
}

func (t *TtyConsole) Close() error {
	t.SlavePty.Close()
	return t.MasterPty.Close()
}

type StdConsole struct {
}

func NewStdConsole(command *Command, pipes *Pipes) (*StdConsole, error) {
	std := &StdConsole{}

	if err := std.AttachPipes(&command.Cmd, pipes); err != nil {
		return nil, err
	}
	return std, nil
}

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

func (s *StdConsole) Resize(h, w int) error {
	// we do not need to reside a non tty
	return nil
}

func (s *StdConsole) Close() error {
	// nothing to close here
	return nil
}
