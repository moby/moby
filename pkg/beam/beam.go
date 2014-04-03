package beam

import (
	"fmt"
	"io"
	"os"
)

type Sender interface {
	Send([]byte, *os.File) error
}

type Receiver interface {
	Receive() ([]byte, *os.File, error)
}

type ReceiveCloser interface {
	Receiver
	Close() error
}

type SendCloser interface {
	Sender
	Close() error
}

type ReceiveSender interface {
	Receiver
	Sender
}

func SendPipe(dst Sender, data []byte) (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	if err := dst.Send(data, r); err != nil {
		r.Close()
		w.Close()
		return nil, err
	}
	return w, nil
}

func SendConn(dst Sender, data []byte) (conn *UnixConn, err error) {
	local, remote, err := SocketPair()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			local.Close()
			remote.Close()
		}
	}()
	conn, err = FileConn(local)
	if err != nil {
		return nil, err
	}
	local.Close()
	if err := dst.Send(data, remote); err != nil {
		return nil, err
	}
	return conn, nil
}

func ReceiveConn(src Receiver) ([]byte, *UnixConn, error) {
	for {
		data, f, err := src.Receive()
		if err != nil {
			return nil, nil, err
		}
		if f == nil {
			// Skip empty attachments
			continue
		}
		conn, err := FileConn(f)
		if err != nil {
			// Skip beam attachments which are not connections
			// (for example might be a regular file, directory etc)
			continue
		}
		return data, conn, nil
	}
	panic("impossibru!")
	return nil, nil, nil
}

func Copy(dst Sender, src Receiver) (int, error) {
	var n int
	for {
		payload, attachment, err := src.Receive()
		if err == io.EOF {
			return n, nil
		} else if err != nil {
			return n, err
		}
		if err := dst.Send(payload, attachment); err != nil {
			if attachment != nil {
				attachment.Close()
			}
			return n, err
		}
		n++
	}
	panic("impossibru!")
	return n, nil
}

// MsgDesc returns a human readable description of a beam message, usually
// for debugging purposes.
func MsgDesc(payload []byte, attachment *os.File) string {
	var filedesc string = "<nil>"
	if attachment != nil {
		filedesc = fmt.Sprintf("%d", attachment.Fd())
	}
	return fmt.Sprintf("'%s'[%s]", payload, filedesc)
}

type devnull struct{}

func Devnull() ReceiveSender {
	return devnull{}
}

func (d devnull) Send(p []byte, a *os.File) error {
	if a != nil {
		a.Close()
	}
	return nil
}

func (d devnull) Receive() ([]byte, *os.File, error) {
	return nil, nil, io.EOF
}
