package lxc

import (
	"github.com/dotcloud/docker/execdriver"
	"github.com/dotcloud/docker/pkg/term"
	"github.com/kr/pty"
	"io"
	"os"
)

func SetTerminal(command *execdriver.Command, pipes *execdriver.Pipes) error {
	var (
		term execdriver.Terminal
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
	master *os.File
	slave  *os.File
}

func NewTtyConsole(command *execdriver.Command, pipes *execdriver.Pipes) (*TtyConsole, error) {
	ptyMaster, ptySlave, err := pty.Open()
	if err != nil {
		return nil, err
	}
	tty := &TtyConsole{
		master: ptyMaster,
		slave:  ptySlave,
	}
	if err := tty.attach(command, pipes); err != nil {
		tty.Close()
		return nil, err
	}
	return tty, nil
}

func (t *TtyConsole) Master() *os.File {
	return t.master
}

func (t *TtyConsole) Resize(h, w int) error {
	return term.SetWinsize(t.master.Fd(), &term.Winsize{Height: uint16(h), Width: uint16(w)})
}

func (t *TtyConsole) attach(command *execdriver.Command, pipes *execdriver.Pipes) error {
	command.Stdout = t.slave
	command.Stderr = t.slave
	command.Console = t.slave.Name()

	go func() {
		if wb, ok := pipes.Stdout.(interface {
			CloseWriters() error
		}); ok {
			defer wb.CloseWriters()
		}
		io.Copy(pipes.Stdout, t.master)
	}()

	if pipes.Stdin != nil {
		command.Stdin = t.slave
		command.SysProcAttr.Setctty = true

		go func() {
			defer pipes.Stdin.Close()
			io.Copy(t.master, pipes.Stdin)
		}()
	}
	return nil
}

func (t *TtyConsole) Close() error {
	t.slave.Close()
	return t.master.Close()
}

type StdConsole struct {
}

func NewStdConsole(command *execdriver.Command, pipes *execdriver.Pipes) (*StdConsole, error) {
	std := &StdConsole{}

	if err := std.attach(command, pipes); err != nil {
		return nil, err
	}
	return std, nil
}

func (s *StdConsole) attach(command *execdriver.Command, pipes *execdriver.Pipes) error {
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
