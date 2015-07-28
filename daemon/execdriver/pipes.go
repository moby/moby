package execdriver

import (
	"io"
)

// Pipes is a wrapper around a container's output for
// stdin, stdout, stderr
type Pipes struct {
	Stdin          io.ReadCloser
	Stdout, Stderr io.Writer
}

// NewPipes returns a wrapper around a container's output
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
