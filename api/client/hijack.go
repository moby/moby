package client

import (
	"io"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/engine-api/types"
)

func (cli *DockerCli) holdHijackedConnection(tty bool, inputStream io.ReadCloser, outputStream, errorStream io.Writer, resp types.HijackedResponse) error {
	var err error
	receiveStdout := make(chan error, 1)
	if outputStream != nil || errorStream != nil {
		go func() {
			// When TTY is ON, use regular copy
			if tty && outputStream != nil {
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
