package pageant

import (
	"io"
	"sync"

	"golang.org/x/crypto/ssh/agent"
)

// New returns new ssh-agent instance (see http://golang.org/x/crypto/ssh/agent)
func New() agent.Agent {
	return agent.NewClient(&conn{})
}

type conn struct {
	sync.Mutex
	buf []byte
}

func (c *conn) Write(p []byte) (int, error) {
	c.Lock()
	defer c.Unlock()

	resp, err := query(p)
	if err != nil {
		return 0, err
	}
	c.buf = append(c.buf, resp...)
	return len(p), nil
}

func (c *conn) Read(p []byte) (int, error) {
	c.Lock()
	defer c.Unlock()

	if len(c.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(p, c.buf)
	c.buf = c.buf[n:]
	return n, nil
}
