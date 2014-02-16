/*
   Package to allow go applications to immediately start
   listening on a socket, unix, tcp, udp but hold connections
   until the application has booted and is ready to accept them
*/
package socketactivation

import (
	"fmt"
	"net"
	"time"
)

// NewActivationListener returns a listener listening on addr with the protocol.  It sets the
// timeout to wait on first connection before an error is returned
func NewActivationListener(proto, addr string, activate chan struct{}, timeout time.Duration) (net.Listener, error) {
	wrapped, err := net.Listen(proto, addr)
	if err != nil {
		return nil, err
	}

	return &defaultListener{
		wrapped:  wrapped,
		activate: activate,
		timeout:  timeout,
	}, nil
}

type defaultListener struct {
	wrapped  net.Listener // the real listener to wrap
	ready    bool         // is the listner ready to start accpeting connections
	activate chan struct{}
	timeout  time.Duration // how long to wait before we consider this an error
}

func (l *defaultListener) Close() error {
	return l.wrapped.Close()
}

func (l *defaultListener) Addr() net.Addr {
	return l.wrapped.Addr()
}

func (l *defaultListener) Accept() (net.Conn, error) {
	// if the listen has been told it is ready then we can go ahead and
	// start returning connections
	if l.ready {
		return l.wrapped.Accept()
	}

	select {
	case <-time.After(l.timeout):
		// close the connection so any clients are disconnected
		l.Close()
		return nil, fmt.Errorf("timeout (%s) reached waiting for listener to become ready", l.timeout.String())
	case <-l.activate:
		l.ready = true
		return l.Accept()
	}
	panic("unreachable")
}
