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
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/go-connections/sockets"
	"github.com/pkg/errors"
)

// tlsClientCon holds tls information and a dialed connection.
type tlsClientCon struct {
	*tls.Conn
	rawConn net.Conn
}

func (c *tlsClientCon) CloseWrite() error {
	// Go standard tls.Conn doesn't provide the CloseWrite() method so we do it
	// on its underlying connection.
	if conn, ok := c.rawConn.(types.CloseWriter); ok {
		return conn.CloseWrite()
	}
	return nil
}

// postHijacked sends a POST request and hijacks the connection.
func (cli *Client) postHijacked(ctx context.Context, path string, query url.Values, body interface{}, headers map[string][]string) (types.HijackedResponse, error) {
	bodyEncoded, err := encodeData(body)
	if err != nil {
		return types.HijackedResponse{}, err
	}

	apiPath := cli.getAPIPath(path, query)
	req, err := http.NewRequest("POST", apiPath, bodyEncoded)
	if err != nil {
		return types.HijackedResponse{}, err
	}
	req = cli.addHeaders(req, headers)
	req = req.WithContext(ctx)

	conn, err := cli.setupHijackConn(req, "tcp")
	if err != nil {
		return types.HijackedResponse{}, err
	}

	return types.HijackedResponse{Conn: conn, Reader: bufio.NewReader(conn)}, err
}

// We need to copy Go's implementation of tls.Dial (pkg/cryptor/tls/tls.go) in
// order to return our custom tlsClientCon struct which holds both the tls.Conn
// object _and_ its underlying raw connection. The rationale for this is that
// we need to be able to close the write end of the connection when attaching,
// which tls.Conn does not provide.
func tlsDialWithDialer(dialer *net.Dialer, network, addr string, config *tls.Config) (net.Conn, error) {
	// We want the Timeout and Deadline values from dialer to cover the
	// whole process: TCP connection and TLS handshake. This means that we
	// also need to start our own timers now.
	timeout := dialer.Timeout

	if !dialer.Deadline.IsZero() {
		deadlineTimeout := time.Until(dialer.Deadline)
		if timeout == 0 || deadlineTimeout < timeout {
			timeout = deadlineTimeout
		}
	}

	var errChannel chan error
	var done chan struct{}

	if timeout != 0 {
		errChannel = make(chan error)
		done = make(chan struct{})
		defer close(done)

		time.AfterFunc(timeout, func() {
			select {
			case errChannel <- context.DeadlineExceeded:
			case <-done:
			}
		})
	}

	proxyDialer, err := sockets.DialerFromEnvironment(dialer)
	if err != nil {
		return nil, err
	}

	var rawConn net.Conn
	if timeout == 0 {
		rawConn, err = proxyDialer.Dial(network, addr)
	} else {
		go func() {
			rawConn, err = proxyDialer.Dial(network, addr)
			select {
			case errChannel <- err:
			case <-done:
				if err == nil {
					rawConn.Close()
				}
			}
		}()

		err = <-errChannel
	}

	if err != nil {
		return nil, err
	}

	// When we set up a TCP connection for hijack, there could be long periods
	// of inactivity (a long running command with no output) that in certain
	// network setups may cause ECONNTIMEOUT, leaving the client in an unknown
	// state. Setting TCP KeepAlive on the socket connection will prohibit
	// ECONNTIMEOUT unless the socket connection truly is broken
	if tcpConn, ok := rawConn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	colonPos := strings.LastIndex(addr, ":")
	if colonPos == -1 {
		colonPos = len(addr)
	}
	hostname := addr[:colonPos]

	// If no ServerName is set, infer the ServerName
	// from the hostname we're connecting to.
	if config.ServerName == "" {
		// Make a copy to avoid polluting argument or default.
		config = tlsConfigClone(config)
		config.ServerName = hostname
	}

	conn := tls.Client(rawConn, config)

	if timeout == 0 {
		err = conn.Handshake()
	} else {
		go func() {
			err := conn.Handshake()
			select {
			case errChannel <- err:
			case <-done:
				// TODO close conn?
			}
		}()

		err = <-errChannel
	}

	if err != nil {
		rawConn.Close()
		return nil, err
	}

	// This is Docker difference with standard's crypto/tls package: returned a
	// wrapper which holds both the TLS and raw connections.
	return &tlsClientCon{conn, rawConn}, nil
}

func dial(ctx context.Context, proto, addr string, tlsConfig *tls.Config) (net.Conn, error) {
	// 0 = no timeout
	var timeout time.Duration

	deadline, ok := ctx.Deadline()
	if ok {
		timeout = time.Until(deadline)
	}

	if tlsConfig != nil && proto != "unix" && proto != "npipe" {
		// Notice this isn't Go standard's tls.Dial function
		return tlsDialWithDialer(&net.Dialer{Timeout: timeout}, proto, addr, tlsConfig)
	}
	if proto == "npipe" {
		if timeout == 0 {
			// Why 32? See issue 8035
			timeout = 32 * time.Second
		}
		return sockets.DialPipe(addr, timeout)
	}
	return net.DialTimeout(proto, addr, timeout)
}

func (cli *Client) setupHijackConn(req *http.Request, proto string) (net.Conn, error) {
	req.Host = cli.addr
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", proto)

	ctx := req.Context()

	conn, err := dial(ctx, cli.proto, cli.addr, resolveTLSConfig(cli.client.Transport))
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
	errCh := make(chan error, 1)
	go func() {
		resp, err := clientconn.Do(req)
		if err != httputil.ErrPersistEOF {
			if err != nil {
				errCh <- err
				return
			}
			if resp.StatusCode != http.StatusSwitchingProtocols {
				resp.Body.Close()
				errCh <- fmt.Errorf("unable to upgrade to %s, received %d", proto, resp.StatusCode)
				return
			}
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	c, br := clientconn.Hijack()
	if br.Buffered() > 0 {
		// If there is buffered content, wrap the connection.  We return an
		// object that implements CloseWrite iff the underlying connection
		// implements it.
		if _, ok := c.(types.CloseWriter); ok {
			c = &hijackedConnCloseWriter{&hijackedConn{c, br}}
		} else {
			c = &hijackedConn{c, br}
		}
	} else {
		br.Reset(nil)
	}

	return c, nil
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
