package client

import (
	"io"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
)

func (cli *DockerCli) holdHijackedConnection(setRawTerminal bool, inputStream io.ReadCloser, outputStream, errorStream io.Writer, resp types.HijackedResponse) error {
	var (
		err      error
		oldState *term.State
	)
	if inputStream != nil && setRawTerminal && cli.isTerminalIn && os.Getenv("NORAW") == "" {
		oldState, err = term.SetRawTerminal(cli.inFd)
		if err != nil {
			return err
		}
		defer term.RestoreTerminal(cli.inFd, oldState)
	}

	receiveStdout := make(chan error, 1)
	if outputStream != nil || errorStream != nil {
		go func() {
			defer func() {
				if inputStream != nil {
					if setRawTerminal && cli.isTerminalIn {
						term.RestoreTerminal(cli.inFd, oldState)
					}
					inputStream.Close()
				}
			}()

			// When TTY is ON, use regular copy
			if setRawTerminal && outputStream != nil {
				_, err = io.Copy(outputStream, resp.Reader)
			} else {
				_, err = stdcopy.StdCopy(outputStream, errorStream, resp.Reader)
			}
			logrus.Debugf("[hijack] End of stdout")
			receiveStdout <- err
		}()
	}

	stdinDone := make(chan struct{})
	go func() {
		if inputStream != nil {
			io.Copy(resp.Conn, inputStream)
			logrus.Debugf("[hijack] End of stdin")
		}

		if err := resp.CloseWrite(); err != nil {
			logrus.Debugf("Couldn't send EOF: %s", err)
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
		if outputStream != nil || errorStream != nil {
			if err := <-receiveStdout; err != nil {
				logrus.Debugf("Error receiveStdout: %s", err)
				return err
			}
		}
	}

	return nil
}
