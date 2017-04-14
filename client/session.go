package client

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// DialSession returns a connection that can be used communication with daemon
func (cli *Client) DialSession(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
	req, err := http.NewRequest("POST", "/session", nil)
	if err != nil {
		return nil, err
	}
	req = cli.addHeaders(req, meta)

	req.Host = cli.addr
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", proto)

	conn, err := dial(cli.proto, cli.addr, resolveTLSConfig(cli.client.Transport))
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to the Docker daemon. Is 'docker daemon' running on this host?")
	}

	// When we set up a TCP connection for hijack, there could be long periods
	// of inactivity (a long running command with no output) that in certain
	// network setups may cause ECONNTIMEOUT, leaving the client in an unknown
	// state. Setting TCP KeepAlive on the socket connection will prohibit
	// ECONNTIMEOUT unless the socket connection truly is broken
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	clientconn := httputil.NewClientConn(conn, nil)
	defer clientconn.Close()

	// Server hijacks the connection, error 'connection closed' expected
	resp, err := clientconn.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("unable to upgrade to %s", proto)
	}

	c, br := clientconn.Hijack()
	if br.Buffered() > 0 {
		// If there is buffered content, wrap the connection
		c = &hijackedConn{c, br}
	} else {
		br.Reset(nil)
	}

	return c, nil
}

type hijackedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *hijackedConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}
