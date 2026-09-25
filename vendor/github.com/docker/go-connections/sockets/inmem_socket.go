package sockets

import (
	"context"
	"net"
	"sync"
)

// inmemAddr is used to satisfy net.Addr for the in-memory socket.
type inmemAddr string

// Network returns the addr string, satisfies net.Addr
func (a inmemAddr) Network() string { return "inmem" }

// String returns the string form
func (a inmemAddr) String() string {
	return string(a)
}

// InmemSocket implements [net.Listener] using in-memory only connections.
type InmemSocket struct {
	chConn  chan net.Conn
	chClose chan struct{}
	addr    inmemAddr
	mu      sync.Mutex
}

// NewInmemSocket creates an in-memory only [net.Listener]. The addr argument
// can be any string, but is used to satisfy the [net.Listener.Addr] part
// of the [net.Listener] interface
func NewInmemSocket(addr string, bufSize int) *InmemSocket {
	return &InmemSocket{
		chConn:  make(chan net.Conn, bufSize),
		chClose: make(chan struct{}),
		addr:    inmemAddr(addr),
	}
}

// Addr returns the socket's addr string to satisfy net.Listener
func (s *InmemSocket) Addr() net.Addr {
	return s.addr
}

// Accept implements the Accept method in the Listener interface; it waits
// for the next call and returns a generic Conn. It returns a [net.ErrClosed]
// if the connection is already closed.
func (s *InmemSocket) Accept() (net.Conn, error) {
	select {
	case conn := <-s.chConn:
		return conn, nil
	case <-s.chClose:
		return nil, net.ErrClosed
	}
}

// Close closes the listener. It will be unavailable for use once closed.
func (s *InmemSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.chClose:
	default:
		close(s.chClose)
	}
	return nil
}

// Dial establishes a connection with the in-memory listener.
//
// The network and addr parameters are accepted for compatibility with
// conventional dialer APIs but are currently ignored.
//
// It is equivalent to calling DialContext with context.Background().
// It returns [net.ErrClosed] if the listener has already been closed.
func (s *InmemSocket) Dial(network, addr string) (net.Conn, error) {
	return s.DialContext(context.Background(), network, addr)
}

// DialContext establishes a connection with the in-memory listener.
//
// The network and addr parameters are accepted for compatibility with
// conventional dialer APIs but are currently ignored.
//
// If ctx is canceled before the connection is established, DialContext
// returns the context error. It returns [net.ErrClosed] if the listener
// has already been closed.
func (s *InmemSocket) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	srvConn, clientConn := net.Pipe()
	select {
	case s.chConn <- srvConn:
		return clientConn, nil
	case <-ctx.Done():
		_ = srvConn.Close()
		_ = clientConn.Close()
		return nil, ctx.Err()
	case <-s.chClose:
		_ = srvConn.Close()
		_ = clientConn.Close()
		return nil, net.ErrClosed
	}
}
