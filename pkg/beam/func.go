package beam

import (
	"fmt"
	"os"
)


type Func func([]byte, Stream) error

func (f Func) Send(data []byte, s Stream) error {
	return f(data, s)
}

func (f Func) Receive() ([]byte, Stream, error) {
	return nil, nil, fmt.Errorf("receive: operation not supported")
}

func (f Func) File() (*os.File, error) {
	return nil, fmt.Errorf("no file descriptor associated with stream")
}

func Close() error {
	return fmt.Errorf("receive: operation not supported")
}
