package timeoutconn

import (
	"net"
	"time"
)

func New(netConn net.Conn, timeout time.Duration) net.Conn {
	return &conn{netConn, timeout}
}

// A net.Conn that sets a deadline for every Read or Write operation
type conn struct {
	net.Conn
	timeout time.Duration
}

func (c *conn) Read(b []byte) (int, error) {
	if c.timeout > 0 {
		err := c.Conn.SetReadDeadline(time.Now().Add(c.timeout))
		if err != nil {
			return 0, err
		}
	}
	return c.Conn.Read(b)
}
