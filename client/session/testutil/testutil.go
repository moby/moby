package testutil

import (
	"io"
	"net"
	"time"

	"github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
)

// Handler is function called to handle incoming connection
type Handler func(ctx context.Context, conn net.Conn, meta map[string][]string) error

// Dialer is a function for dialing an outgoing connection
type Dialer func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error)

// TestStream creates an in memory session dialer for a handler function
func TestStream(handler Handler) Dialer {
	s1, s2 := sockPair()
	return func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
		go func() {
			err := handler(context.TODO(), s1, meta)
			if err != nil {
				logrus.Error(err)
			}
			s1.Close()
		}()
		return s2, nil
	}
}

func sockPair() (*sock, *sock) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	return &sock{pw1, pr2, pw1}, &sock{pw2, pr1, pw2}
}

type sock struct {
	io.Writer
	io.Reader
	io.Closer
}

func (s *sock) LocalAddr() net.Addr {
	return dummyAddr{}
}
func (s *sock) RemoteAddr() net.Addr {
	return dummyAddr{}
}
func (s *sock) SetDeadline(t time.Time) error {
	return nil
}
func (s *sock) SetReadDeadline(t time.Time) error {
	return nil
}
func (s *sock) SetWriteDeadline(t time.Time) error {
	return nil
}

type dummyAddr struct {
}

func (d dummyAddr) Network() string {
	return "tcp"
}

func (d dummyAddr) String() string {
	return "localhost"
}
