package beam

import (
	"fmt"
	"io"
	"os"
)

type Func func(Message) error

func (f Func) Send(msg Message) error {
	go func() {
		f(msg)
		if msg.Stream != nil {
			msg.Stream.Close()
		}
	}()
	return nil
}

func (f Func) Receive() (msg Message, err error) {
	err = io.EOF
	return
}

func (f Func) File() (*os.File, error) {
	return nil, fmt.Errorf("no file descriptor associated with stream")
}

func (f Func) Close() error {
	return fmt.Errorf("receive: operation not supported")
}
