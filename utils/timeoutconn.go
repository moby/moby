package utils

import (
	"net"
	"time"
)

func NewTimeoutConn(conn net.Conn, timeout time.Duration) net.Conn {
	return &TimeoutConn{conn, timeout}
}

// A net.Conn that sets a deadline for every Read or Write operation
type TimeoutConn struct {
	net.Conn
	timeout time.Duration
}

func (c *TimeoutConn) Read(b []byte) (int, error) {
	if c.timeout > 0 {
		err := c.Conn.SetReadDeadline(time.Now().Add(c.timeout))
		if err != nil {
			return 0, err
		}
	}
	return c.Conn.Read(b)
}
