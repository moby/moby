package beam

import (
	"net"
)

// Listen is a convenience interface for applications to create service endpoints
// which can be easily used with existing networking code.
//
// Listen registers a new service endpoint on the beam connection `conn`, using the
// service name `name`. It returns a listener which can be used in the usual
// way. Calling Accept() on the listener will block until a new connection is available
// on the service endpoint. The endpoint is then returned as a regular net.Conn and
// can be used as a regular network connection.
//
// Note that if the underlying file descriptor received in attachment is nil or does
// not point to a connection, that message will be skipped.
//
func Listen(conn Sender, name string) (net.Listener, error) {
	endpoint, err := SendConn(conn, []byte(name))
	if err != nil {
		return nil, err
	}
	return &listener{
		name:     name,
		endpoint: endpoint,
	}, nil
}

func Connect(ctx *UnixConn, name string) (net.Conn, error) {
	l, err := Listen(ctx, name)
	if err != nil {
		return nil, err
	}
	conn, err := l.Accept()
	if err != nil {
		return nil, err
	}
	return conn, nil
}

type listener struct {
	name     string
	endpoint ReceiveCloser
}

func (l *listener) Accept() (net.Conn, error) {
	for {
		_, f, err := l.endpoint.Receive()
		if err != nil {
			return nil, err
		}
		if f == nil {
			// Skip empty attachments
			continue
		}
		conn, err := net.FileConn(f)
		if err != nil {
			// Skip beam attachments which are not connections
			// (for example might be a regular file, directory etc)
			continue
		}
		return conn, nil
	}
	panic("impossibru!")
	return nil, nil
}

func (l *listener) Close() error {
	return l.endpoint.Close()
}

func (l *listener) Addr() net.Addr {
	return addr(l.name)
}

type addr string

func (a addr) Network() string {
	return "beam"
}

func (a addr) String() string {
	return string(a)
}
