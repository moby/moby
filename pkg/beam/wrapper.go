package beam

import (
	"fmt"
	"io"
	"os"
)


type ioWrapper struct {
	obj interface{}
	blocksize int
}

func WrapIO(obj interface{}, blocksize int) Stream {
	wrapper := ioWrapper{obj, blocksize}
	if blocksize == 0 {
		wrapper.blocksize = 4096
	}
	return wrapper
}

func (w ioWrapper) Receive() (data []byte, s Stream, err error) {
	reader, ok := w.obj.(io.Reader)
	if !ok {
		// Return EOF if Read is not implemented
		return nil, nil, io.EOF
	}
	data = make([]byte, w.blocksize)
	n, err := reader.Read(data)
	return data[:n], nil, err
}

func (w ioWrapper) Send(data []byte, s Stream) error {
	writer, ok := w.obj.(io.Writer)
	if !ok {
		// Silently discard the data if Write is not implemented
		return nil
	}
	if s != nil {
		return fmt.Errorf("send stream: operation not supported")
	}
	_, err := writer.Write(data)
	return err
}

func (w ioWrapper) Close() error {
	closer, ok := w.obj.(io.Closer)
	if !ok {
		return fmt.Errorf("close: operation not supported")
	}
	return closer.Close()
}

func (w ioWrapper) File() (*os.File, error) {
	filer, ok := w.obj.(interface { File()(*os.File, error) })
	if !ok {
		return nil, fmt.Errorf("no file descriptor associated with stream")
	}
	return filer.File()
}
