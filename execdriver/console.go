package execdriver

import (
	"github.com/dotcloud/docker/pkg/term"
	"github.com/kr/pty"
	"io"
	"os"
)

type Console interface {
	io.Closer
	Resize(height, width int) error
	AttachTo(command *Command) error
}

type pipes struct {
	Stdin          io.ReadCloser
	Stdout, Stderr io.WriteCloser
}

func (p *pipes) Close() error {
	if p.Stderr != nil {
		p.Stdin.Close()
	}
	if p.Stdout != nil {
		p.Stdout.Close()
	}
	if p.Stderr != nil {
		p.Stderr.Close()
	}
	return nil
}

func NewConsole(stdin io.ReadCloser, stdout, stderr io.WriteCloser, useStdin, tty bool) (Console, error) {
	p := &pipes{
		Stdout: stdout,
		Stderr: stderr,
	}
	if useStdin {
		p.Stdin = stdin
	}
	if tty {
		return NewTtyConsole(p)
	}
	return NewStdConsole(p)
}

type TtyConsole struct {
	Master *os.File
	Slave  *os.File
	pipes  *pipes
}

func NewTtyConsole(p *pipes) (*TtyConsole, error) {
	ptyMaster, ptySlave, err := pty.Open()
	if err != nil {
		return nil, err
	}
	tty := &TtyConsole{
		Master: ptyMaster,
		Slave:  ptySlave,
		pipes:  p,
	}
	return tty, nil
}

func (t *TtyConsole) Resize(h, w int) error {
	return term.SetWinsize(t.Master.Fd(), &term.Winsize{Height: uint16(h), Width: uint16(w)})

}

func (t *TtyConsole) AttachTo(command *Command) error {
	command.Stdout = t.Slave
	command.Stderr = t.Slave

	command.Console = t.Slave.Name()

	go func() {
		defer t.pipes.Stdout.Close()
		io.Copy(t.pipes.Stdout, t.Master)
	}()

	if t.pipes.Stdin != nil {
		command.Stdin = t.Slave
		command.SysProcAttr.Setctty = true

		go func() {
			defer t.pipes.Stdin.Close()
			io.Copy(t.Master, t.pipes.Stdin)
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
	pipes *pipes
}

func NewStdConsole(p *pipes) (*StdConsole, error) {
	return &StdConsole{p}, nil
}

func (s *StdConsole) AttachTo(command *Command) error {
	command.Stdout = s.pipes.Stdout
	command.Stderr = s.pipes.Stderr

	if s.pipes.Stdin != nil {
		stdin, err := command.StdinPipe()
		if err != nil {
			return err
		}

		go func() {
			defer stdin.Close()
			io.Copy(stdin, s.pipes.Stdin)
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
