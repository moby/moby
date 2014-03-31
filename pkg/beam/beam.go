package beam

import (
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
