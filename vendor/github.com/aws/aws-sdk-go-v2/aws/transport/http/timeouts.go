package http

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"time"
)

// deadlineConn applies a rolling inactivity window to reads on a connection by
// resetting the read deadline before each one. A read returns as soon as any
// bytes are available, so a slow but progressing transfer survives, and a
// connection that goes silent fails once.
type deadlineConn struct {
	net.Conn
	timeout time.Duration
}

// Read implements [io.Reader].
func (c *deadlineConn) Read(p []byte) (int, error) {
	if err := c.Conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}

	n, err := c.Conn.Read(p)
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return n, &ResponseTimeoutError{TimeoutDur: c.timeout}
	}

	return n, err
}

func (b *BuildableClient) installReadTimeout(tr *http.Transport) {
	timeout, ok := b.GetReadTimeout()
	if !ok || timeout <= 0 {
		return
	}

	dial := tr.DialContext
	if dial == nil {
		dial = defaultDialer().DialContext
	}

	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dial(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		return &deadlineConn{Conn: conn, timeout: timeout}, nil
	}
}
