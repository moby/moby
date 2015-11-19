package client

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
)

type tlsClientCon struct {
	*tls.Conn
	rawConn net.Conn
}

func (c *tlsClientCon) CloseWrite() error {
	// Go standard tls.Conn doesn't provide the CloseWrite() method so we do it
	// on its underlying connection.
	if cwc, ok := c.rawConn.(interface {
		CloseWrite() error
	}); ok {
		return cwc.CloseWrite()
	}
	return nil
}

func tlsDial(network, addr string, config *tls.Config) (net.Conn, error) {
	return tlsDialWithDialer(new(net.Dialer), network, addr, config)
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
		deadlineTimeout := dialer.Deadline.Sub(time.Now())
		if timeout == 0 || deadlineTimeout < timeout {
			timeout = deadlineTimeout
		}
	}

	var errChannel chan error

	if timeout != 0 {
		errChannel = make(chan error, 2)
		time.AfterFunc(timeout, func() {
			errChannel <- errors.New("")
		})
	}

	rawConn, err := dialer.Dial(network, addr)
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
		c := *config
		c.ServerName = hostname
		config = &c
	}

	conn := tls.Client(rawConn, config)

	if timeout == 0 {
		err = conn.Handshake()
	} else {
		go func() {
			errChannel <- conn.Handshake()
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

func (cli *DockerCli) dial() (net.Conn, error) {
	if cli.tlsConfig != nil && cli.proto != "unix" {
		// Notice this isn't Go standard's tls.Dial function
		return tlsDial(cli.proto, cli.addr, cli.tlsConfig)
	}
	return net.Dial(cli.proto, cli.addr)
}

func (cli *DockerCli) hijack(method, path string, setRawTerminal bool, in io.ReadCloser, stdout, stderr io.Writer, started chan io.Closer, data interface{}) error {
	return cli.hijackWithContentType(method, path, "text/plain", setRawTerminal, in, stdout, stderr, started, data)
}

func (cli *DockerCli) hijackWithContentType(method, path, contentType string, setRawTerminal bool, in io.ReadCloser, stdout, stderr io.Writer, started chan io.Closer, data interface{}) error {
	defer func() {
		if started != nil {
			close(started)
		}
	}()

	params, err := cli.encodeData(data)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, fmt.Sprintf("%s/v%s%s", cli.basePath, api.Version, path), params)
	if err != nil {
		return err
	}

	// Add CLI Config's HTTP Headers BEFORE we set the Docker headers
	// then the user can't change OUR headers
	for k, v := range cli.configFile.HTTPHeaders {
		req.Header.Set(k, v)
	}

	req.Header.Set("User-Agent", "Docker-Client/"+dockerversion.Version+" ("+runtime.GOOS+")")
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "tcp")
	req.Host = cli.addr

	dial, err := cli.dial()
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("Cannot connect to the Docker daemon. Is 'docker daemon' running on this host?")
		}
		return err
	}

	// When we set up a TCP connection for hijack, there could be long periods
	// of inactivity (a long running command with no output) that in certain
	// network setups may cause ECONNTIMEOUT, leaving the client in an unknown
	// state. Setting TCP KeepAlive on the socket connection will prohibit
	// ECONNTIMEOUT unless the socket connection truly is broken
	if tcpConn, ok := dial.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	clientconn := httputil.NewClientConn(dial, nil)
	defer clientconn.Close()

	// Server hijacks the connection, error 'connection closed' expected
	clientconn.Do(req)

	rwc, br := clientconn.Hijack()
	defer rwc.Close()

	if started != nil {
		started <- rwc
	}

	var oldState *term.State

	if in != nil && setRawTerminal && cli.isTerminalIn && os.Getenv("NORAW") == "" {
		oldState, err = term.SetRawTerminal(cli.inFd)
		if err != nil {
			return err
		}
		defer term.RestoreTerminal(cli.inFd, oldState)
	}

	receiveStdout := make(chan error, 1)
	if stdout != nil || stderr != nil {
		go func() {
			defer func() {
				if in != nil {
					if setRawTerminal && cli.isTerminalIn {
						term.RestoreTerminal(cli.inFd, oldState)
					}
					in.Close()
				}
			}()

			// When TTY is ON, use regular copy
			if setRawTerminal && stdout != nil {
				_, err = io.Copy(stdout, br)
			} else {
				_, err = stdcopy.StdCopy(stdout, stderr, br)
			}
			logrus.Debugf("[hijack] End of stdout")
			receiveStdout <- err
		}()
	}

	stdinDone := make(chan struct{})
	go func() {
		if in != nil {
			io.Copy(rwc, in)
			logrus.Debugf("[hijack] End of stdin")
		}

		if conn, ok := rwc.(interface {
			CloseWrite() error
		}); ok {
			if err := conn.CloseWrite(); err != nil {
				logrus.Debugf("Couldn't send EOF: %s", err)
			}
		}
		close(stdinDone)
	}()

	select {
	case err := <-receiveStdout:
		if err != nil {
			logrus.Debugf("Error receiveStdout: %s", err)
			return err
		}
	case <-stdinDone:
		if stdout != nil || stderr != nil {
			if err := <-receiveStdout; err != nil {
				logrus.Debugf("Error receiveStdout: %s", err)
				return err
			}
		}
	}

	return nil
}
