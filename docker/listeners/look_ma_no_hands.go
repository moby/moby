// +build !windows

package listeners

import (
	"fmt"
	"net"
	"strings"
)

type LookMaNoHands struct {
	net.Listener
}

type LookMaNoHandsConn struct {
	net.Conn
	first bool
}

func (l *LookMaNoHandsConn) Read(b []byte) (n int, err error) {
	// yeah, http.Server uses a 4k buffer
	if l.first && len(b) == 4096 {
		l.first = false
		c, err := l.Conn.Read(b[:1048])
		if err != nil {
			return c, err
		}

		fmt.Println("!!!WAS!!!", string(b[:c]))

		parts := strings.Split(string(b[:c]), "\n")
		hackedUp := []string{parts[0]}

		// Sanitize Host header
		hackedUp = append(hackedUp, "Host: docker")

		// Inject `Connection: close` to ensure we don't reuse this connection
		hackedUp = append(hackedUp, "Connection: close")

		for i, leftOver := range parts {
			if i > 1 {
				hackedUp = append(hackedUp, string(leftOver))
			}
		}
		newContent := strings.Join(hackedUp, "\n")

		fmt.Println("!!!NOW!!!", newContent)

		copy(b, []byte(newContent))
		return len(newContent), nil
	}

	return l.Conn.Read(b)
}

func (l *LookMaNoHands) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return c, err
	}
	return &LookMaNoHandsConn{c, true}, nil
}
