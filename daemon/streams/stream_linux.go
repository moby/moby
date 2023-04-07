package streams

import (
	"io"
	"os"

	"github.com/cpuguy83/pipes"
)

func openPipe(path string) (io.ReadCloser, io.WriteCloser, error) {
	flags := os.O_RDWR | os.O_CREATE
	return pipes.OpenFifo(path, flags, 0o660)
}
