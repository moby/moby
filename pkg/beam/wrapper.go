package beam

import (
	"fmt"
	"io"
	"os"
)

type ioWrapper struct {
	obj       interface{}
	blocksize int
}

// WrapIO receives a value `obj` as argument, and returns a
// Stream `s` with the following properties:
//
//   * s.Receive() calls obj.Read() of `blocksize` bytes and returns the result as data.
//   * s.Send() calls obj.Write() with the data of the message to send.
//   * s.Close() calls obj.Close().
//
// If `obj` does not implement some of these methods, the respective calls
// to `s` simply do nothing.
//
// Example:
//
// // Return Message{Data: []byte("hello")}
// WrapIO(bytes.NewBufferString("hello"), 0).Receive()
//
func WrapIO(obj interface{}, blocksize int) Stream {
	wrapper := ioWrapper{obj, blocksize}
	if blocksize == 0 {
		wrapper.blocksize = 4096
	}
	return wrapper
}

func (w ioWrapper) Receive() (msg Message, err error) {
	reader, ok := w.obj.(io.Reader)
	if !ok {
		// Return EOF if Read is not implemented
		err = io.EOF
		return
	}
	var n int
	data := make([]byte, w.blocksize)
	n, err = reader.Read(data)
	msg.Data = data[:n]
	return
}

func (w ioWrapper) Send(msg Message) error {
	writer, ok := w.obj.(io.Writer)
	if !ok {
		// Silently discard the data if Write is not implemented
		return nil
	}
	if msg.Stream != nil {
		return fmt.Errorf("send stream: operation not supported")
	}
	_, err := writer.Write(msg.Data)
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
	filer, ok := w.obj.(interface {
		File() (*os.File, error)
	})
	if !ok {
		return nil, fmt.Errorf("no file descriptor associated with stream")
	}
	return filer.File()
}
