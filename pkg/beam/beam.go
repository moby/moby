package beam

import (
	"os"
)

type Stream interface {
	Send(Message) error
	Receive() (Message, error)

	File() (*os.File, error)
	Close() error
}

type Message struct {
	Data   []byte
	Stream Stream
}
