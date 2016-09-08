package pool

import "net"

// PoolConn is a wrapper around net.Conn to modify the the behavior of
// net.Conn's Close() method.
type PoolConn struct {
	net.Conn
	c        *channelPool
	unusable bool
}

// Close() puts the given connects back to the pool instead of closing it.
func (p *PoolConn) Close() error {
	if p.unusable {
		if p.Conn != nil {
			return p.Conn.Close()
		}
		return nil
	}
	return p.c.put(p.Conn)
}

// MarkUnusable() marks the connection not usable any more, to let the pool close it instead of returning it to pool.
func (p *PoolConn) MarkUnusable() {
	p.unusable = true
}

// newConn wraps a standard net.Conn to a poolConn net.Conn.
func (c *channelPool) wrapConn(conn net.Conn) net.Conn {
	p := &PoolConn{c: c}
	p.Conn = conn
	return p
}
