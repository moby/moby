package beam

import (
	"os"
)

type Stream interface {
	Send(b []byte, s Stream) error
	Receive() ([]byte, Stream, error)

	File() (*os.File, error)
	Close() error
}
