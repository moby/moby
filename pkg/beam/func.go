package beam

import (
	"io"
	"fmt"
	"os"
)


type Func func([]byte, Stream) error

func (f Func) Send(data []byte, s Stream) error {
	go func() {
		f(data, s)
		if s != nil {
			s.Close()
		}
	}()
	return nil
}

func (f Func) Receive() ([]byte, Stream, error) {
	return nil, nil, io.EOF
}

func (f Func) File() (*os.File, error) {
	return nil, fmt.Errorf("no file descriptor associated with stream")
}



func (f Func) Close() error {
	return fmt.Errorf("receive: operation not supported")
}
