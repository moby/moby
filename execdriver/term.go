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
}

type Pipes struct {
	Stdin          io.ReadCloser
	Stdout, Stderr io.Writer
}

func NewPipes(stdin io.ReadCloser, stdout, stderr io.Writer, useStdin bool) *Pipes {
	p := &Pipes{
		Stdout: stdout,
		Stderr: stderr,
	}
	if useStdin {
		p.Stdin = stdin
	}
	return p
}

func SetTerminal(command *Command, pipes *Pipes) error {
	var (
		term Term
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
	Master *os.File
	Slave  *os.File
}

func NewTtyConsole(command *Command, pipes *Pipes) (*TtyConsole, error) {
	ptyMaster, ptySlave, err := pty.Open()
	if err != nil {
		return nil, err
	}
	tty := &TtyConsole{
		Master: ptyMaster,
		Slave:  ptySlave,
	}
	if err := tty.attach(command, pipes); err != nil {
		tty.Close()
		return nil, err
	}
	return tty, nil
}

func (t *TtyConsole) Resize(h, w int) error {
	return term.SetWinsize(t.Master.Fd(), &term.Winsize{Height: uint16(h), Width: uint16(w)})
}

func (t *TtyConsole) attach(command *Command, pipes *Pipes) error {
	command.Stdout = t.Slave
	command.Stderr = t.Slave
	command.Console = t.Slave.Name()

	go func() {
		if wb, ok := pipes.Stdout.(interface {
			CloseWriters() error
		}); ok {
			defer wb.CloseWriters()
		}
		io.Copy(pipes.Stdout, t.Master)
	}()

	if pipes.Stdin != nil {
		command.Stdin = t.Slave
		command.SysProcAttr.Setctty = true

		go func() {
			defer pipes.Stdin.Close()
			io.Copy(t.Master, pipes.Stdin)
		}()
	}
	return nil
}

func (t *TtyConsole) Close() error {
	t.Slave.Close()
	return t.Master.Close()
}

type StdConsole struct {
}

func NewStdConsole(command *Command, pipes *Pipes) (*StdConsole, error) {
	std := &StdConsole{}

	if err := std.attach(command, pipes); err != nil {
		return nil, err
	}
	return std, nil
}

func (s *StdConsole) attach(command *Command, pipes *Pipes) error {
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
