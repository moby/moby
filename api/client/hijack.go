package client

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime"
	"strings"

	"github.com/dotcloud/docker/api"
	"github.com/dotcloud/docker/dockerversion"
	"github.com/dotcloud/docker/pkg/term"
	"github.com/dotcloud/docker/utils"
)

func (cli *DockerCli) dial() (net.Conn, error) {
	return cli.Remote.Dial(cli)
}

func (cli *DockerCli) hijack(method, path string, setRawTerminal bool, in io.ReadCloser, stdout, stderr io.Writer, started chan io.Closer) error {
	defer func() {
		if started != nil {
			close(started)
		}
	}()

	req, err := http.NewRequest(method, fmt.Sprintf("/v%s%s", api.APIVERSION, path), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Docker-Client/"+dockerversion.VERSION)
	req.Header.Set("Content-Type", "plain/text")
	req.Host = cli.Address

	dial, err := cli.dial()
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("Cannot connect to the Docker daemon. Is 'docker -d' running on this host?")
		}
		return err
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

	var receiveStdout chan error

	var oldState *term.State

	if in != nil && setRawTerminal && cli.IsTerminal && os.Getenv("NORAW") == "" {
		oldState, err = term.SetRawTerminal(cli.TerminalFd)
		if err != nil {
			return err
		}
		defer term.RestoreTerminal(cli.TerminalFd, oldState)
	}

	if stdout != nil || stderr != nil {
		receiveStdout = utils.Go(func() (err error) {
			defer func() {
				if in != nil {
					if setRawTerminal && cli.IsTerminal {
						term.RestoreTerminal(cli.TerminalFd, oldState)
					}
					// For some reason this Close call blocks on darwin..
					// As the client exists right after, simply discard the close
					// until we find a better solution.
					if runtime.GOOS != "darwin" {
						in.Close()
					}
				}
			}()

			// When TTY is ON, use regular copy
			if setRawTerminal {
				_, err = io.Copy(stdout, br)
			} else {
				_, err = utils.StdCopy(stdout, stderr, br)
			}
			utils.Debugf("[hijack] End of stdout")
			return err
		})
	}

	sendStdin := utils.Go(func() error {
		if in != nil {
			io.Copy(rwc, in)
			utils.Debugf("[hijack] End of stdin")
		}
		if tcpc, ok := rwc.(*net.TCPConn); ok {
			if err := tcpc.CloseWrite(); err != nil {
				utils.Debugf("Couldn't send EOF: %s\n", err)
			}
		} else if unixc, ok := rwc.(*net.UnixConn); ok {
			if err := unixc.CloseWrite(); err != nil {
				utils.Debugf("Couldn't send EOF: %s\n", err)
			}
		}
		// Discard errors due to pipe interruption
		return nil
	})

	if stdout != nil || stderr != nil {
		if err := <-receiveStdout; err != nil {
			utils.Debugf("Error receiveStdout: %s", err)
			return err
		}
	}

	if !cli.IsTerminal {
		if err := <-sendStdin; err != nil {
			utils.Debugf("Error sendStdin: %s", err)
			return err
		}
	}
	return nil
}
