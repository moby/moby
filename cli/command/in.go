package command

import (
	"errors"
	"io"
	"os"
	"runtime"

	"github.com/docker/docker/pkg/term"
)

// InStream is an input stream used by the DockerCli to read user input
type InStream struct {
	CommonStream
	in io.ReadCloser
}

func (i *InStream) Read(p []byte) (int, error) {
	return i.in.Read(p)
}

// Close implements the Closer interface
func (i *InStream) Close() error {
	return i.in.Close()
}

// SetRawTerminal sets raw mode on the input terminal
func (i *InStream) SetRawTerminal() (err error) {
	if os.Getenv("NORAW") != "" || !i.CommonStream.isTerminal {
		return nil
	}
	i.CommonStream.state, err = term.SetRawTerminal(i.CommonStream.fd)
	return err
}

// CheckTty checks if we are trying to attach to a container tty
// from a non-tty client input stream, and if so, returns an error.
func (i *InStream) CheckTty(attachStdin, ttyMode bool) error {
	// In order to attach to a container tty, input stream for the client must
	// be a tty itself: redirecting or piping the client standard input is
	// incompatible with `docker run -t`, `docker exec -t` or `docker attach`.
	if ttyMode && attachStdin && !i.isTerminal {
		eText := "the input device is not a TTY"
		if runtime.GOOS == "windows" {
			return errors.New(eText + ".  If you are using mintty, try prefixing the command with 'winpty'")
		}
		return errors.New(eText)
	}
	return nil
}

// NewInStream returns a new InStream object from a ReadCloser
func NewInStream(in io.ReadCloser) *InStream {
	fd, isTerminal := term.GetFdInfo(in)
	return &InStream{CommonStream: CommonStream{fd: fd, isTerminal: isTerminal}, in: in}
}
