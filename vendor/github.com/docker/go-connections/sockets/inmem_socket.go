package sockets

import (
	"net"
	"sync"
)

// dummyAddr is used to satisfy net.Addr for the in-mem socket
// it is just stored as a string and returns the string for all calls
type dummyAddr string

// Network returns the addr string, satisfies net.Addr
func (a dummyAddr) Network() string {
	return string(a)
}

// String returns the string form
func (a dummyAddr) String() string {
	return string(a)
}

// InmemSocket implements [net.Listener] using in-memory only connections.
type InmemSocket struct {
	chConn  chan net.Conn
	chClose chan struct{}
	addr    dummyAddr
	mu      sync.Mutex
}

// NewInmemSocket creates an in-memory only [net.Listener]. The addr argument
// can be any string, but is used to satisfy the [net.Listener.Addr] part
// of the [net.Listener] interface
func NewInmemSocket(addr string, bufSize int) *InmemSocket {
	return &InmemSocket{
		chConn:  make(chan net.Conn, bufSize),
		chClose: make(chan struct{}),
		addr:    dummyAddr(addr),
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

// Dial is used to establish a connection with the in-mem server.
// It returns a [net.ErrClosed] if the connection is already closed.
func (s *InmemSocket) Dial(network, addr string) (net.Conn, error) {
	srvConn, clientConn := net.Pipe()
	select {
	case s.chConn <- srvConn:
	case <-s.chClose:
		return nil, net.ErrClosed
	}

	return clientConn, nil
}
