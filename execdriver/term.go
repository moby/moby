package execdriver

import (
	"github.com/dotcloud/docker/pkg/term"
	"github.com/kr/pty"
	"io"
	"os"
)

type Term interface {
	io.Closer
	Resize(height, width int) error
	Attach(pipes *Pipes) error
}

type Pipes struct {
	Stdin          io.ReadCloser
	Stdout, Stderr io.WriteCloser
}

func NewPipes(stdin io.ReadCloser, stdout, stderr io.WriteCloser, useStdin bool) *Pipes {
	p := &Pipes{
		Stdout: stdout,
		Stderr: stderr,
	}
	if useStdin {
		p.Stdin = stdin
	}
	return p
}

func NewTerminal(command *Command) error {
	var (
		term Term
		err  error
	)
	if command.Tty {
		term, err = NewTtyConsole(command)
	} else {
		term, err = NewStdConsole(command)
	}
	if err != nil {
		return err
	}
	command.Terminal = term
	return nil
}

type TtyConsole struct {
	command *Command
	Master  *os.File
	Slave   *os.File
}

func NewTtyConsole(command *Command) (*TtyConsole, error) {
	ptyMaster, ptySlave, err := pty.Open()
	if err != nil {
		return nil, err
	}
	tty := &TtyConsole{
		Master:  ptyMaster,
		Slave:   ptySlave,
		command: command,
	}
	return tty, nil
}

func (t *TtyConsole) Resize(h, w int) error {
	return term.SetWinsize(t.Master.Fd(), &term.Winsize{Height: uint16(h), Width: uint16(w)})
}

func (t *TtyConsole) Attach(pipes *Pipes) error {
	t.command.Stdout = t.Slave
	t.command.Stderr = t.Slave

	t.command.Console = t.Slave.Name()

	go func() {
		defer pipes.Stdout.Close()
		io.Copy(pipes.Stdout, t.Master)
	}()

	if pipes.Stdin != nil {
		t.command.Stdin = t.Slave
		t.command.SysProcAttr.Setctty = true

		go func() {
			defer pipes.Stdin.Close()
			io.Copy(t.Master, pipes.Stdin)
		}()
	}
	return nil
}

func (t *TtyConsole) Close() error {
	err := t.Slave.Close()
	if merr := t.Master.Close(); err == nil {
		err = merr
	}
	return err
}

type StdConsole struct {
	command *Command
}

func NewStdConsole(command *Command) (*StdConsole, error) {
	return &StdConsole{command}, nil
}

func (s *StdConsole) Attach(pipes *Pipes) error {
	s.command.Stdout = pipes.Stdout
	s.command.Stderr = pipes.Stderr

	if pipes.Stdin != nil {
		stdin, err := s.command.StdinPipe()
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
