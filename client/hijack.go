package client // import "github.com/docker/docker/client"

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/go-connections/sockets"
	"github.com/pkg/errors"
)

// postHijacked sends a POST request and hijacks the connection.
func (cli versionedClient) postHijacked(ctx context.Context, path string, query url.Values, body interface{}, headers map[string][]string) (types.HijackedResponse, error) {
	bodyEncoded, err := encodeData(body)
	if err != nil {
		return types.HijackedResponse{}, err
	}

	apiPath := cli.getAPIPath(path, query)
	req, err := http.NewRequest(http.MethodPost, apiPath, bodyEncoded)
	if err != nil {
		return types.HijackedResponse{}, err
	}
	req = cli.addHeaders(req, headers)

	conn, mediaType, err := cli.setupHijackConn(ctx, req, "tcp")
	if err != nil {
		return types.HijackedResponse{}, err
	}

	return types.NewHijackedResponse(conn, mediaType), err
}

func (cli *Client) postHijacked(ctx context.Context, path string, query url.Values, body interface{}, headers map[string][]string) (types.HijackedResponse, error) {
	versioned, err := cli.versioned(ctx)
	if err != nil {
		return types.HijackedResponse{}, err
	}
	return versioned.postHijacked(ctx, path, query, body, headers)
}

// DialHijack returns a hijacked connection with negotiated protocol proto.
func (cli *Client) DialHijack(ctx context.Context, url, proto string, meta map[string][]string) (net.Conn, error) {
	versioned, err := cli.versioned(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	req = versioned.addHeaders(req, meta)

	conn, _, err := versioned.setupHijackConn(ctx, req, proto)
	return conn, err
}

// fallbackDial is used when WithDialer() was not called.
// See cli.Dialer().
func fallbackDial(proto, addr string, tlsConfig *tls.Config) (net.Conn, error) {
	if tlsConfig != nil && proto != "unix" && proto != "npipe" {
		return tls.Dial(proto, addr, tlsConfig)
	}
	if proto == "npipe" {
		return sockets.DialPipe(addr, 32*time.Second)
	}
	return net.Dial(proto, addr)
}

func (cli versionedClient) setupHijackConn(ctx context.Context, req *http.Request, proto string) (net.Conn, string, error) {
	req.Host = cli.cli.addr
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", proto)

	dialer := cli.cli.Dialer()
	conn, err := dialer(ctx)
	if err != nil {
		return nil, "", errors.Wrap(err, "cannot connect to the Docker daemon. Is 'docker daemon' running on this host?")
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

	//nolint:staticcheck // ignore SA1019 for connecting to old (pre go1.8) daemons
	if err != httputil.ErrPersistEOF {
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode != http.StatusSwitchingProtocols {
			resp.Body.Close()
			return nil, "", fmt.Errorf("unable to upgrade to %s, received %d", proto, resp.StatusCode)
		}
	}

	c, br := clientconn.Hijack()
	if br.Buffered() > 0 {
		// If there is buffered content, wrap the connection.  We return an
		// object that implements CloseWrite if the underlying connection
		// implements it.
		if _, ok := c.(types.CloseWriter); ok {
			c = &hijackedConnCloseWriter{&hijackedConn{c, br}}
		} else {
			c = &hijackedConn{c, br}
		}
	} else {
		br.Reset(nil)
	}

	var mediaType string
	if versions.GreaterThanOrEqualTo(cli.version, "1.42") {
		// Prior to 1.42, Content-Type is always set to raw-stream and not relevant
		mediaType = resp.Header.Get("Content-Type")
	}

	return c, mediaType, nil
}

// hijackedConn wraps a net.Conn and is returned by setupHijackConn in the case
// that a) there was already buffered data in the http layer when Hijack() was
// called, and b) the underlying net.Conn does *not* implement CloseWrite().
// hijackedConn does not implement CloseWrite() either.
type hijackedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *hijackedConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

// hijackedConnCloseWriter is a hijackedConn which additionally implements
// CloseWrite().  It is returned by setupHijackConn in the case that a) there
// was already buffered data in the http layer when Hijack() was called, and b)
// the underlying net.Conn *does* implement CloseWrite().
type hijackedConnCloseWriter struct {
	*hijackedConn
}

var _ types.CloseWriter = &hijackedConnCloseWriter{}

func (c *hijackedConnCloseWriter) CloseWrite() error {
	conn := c.Conn.(types.CloseWriter)
	return conn.CloseWrite()
}
