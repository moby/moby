package daemon

import (
	"fmt"
	"io"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/logger"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/stdcopy"
)

// ContainerAttach attaches to logs according to the config passed in. See ContainerAttachConfig.
func (daemon *Daemon) ContainerAttach(prefixOrName string, c *backend.ContainerAttachConfig) error {
	container, err := daemon.GetContainer(prefixOrName)
	if err != nil {
		return derr.ErrorCodeNoSuchContainer.WithArgs(prefixOrName)
	}
	if container.IsPaused() {
		return derr.ErrorCodePausedContainer.WithArgs(prefixOrName)
	}

	inStream, outStream, errStream, err := c.GetStreams()
	if err != nil {
		return err
	}
	defer inStream.Close()

	if !container.Config.Tty && c.MuxStreams {
		errStream = stdcopy.NewStdWriter(errStream, stdcopy.Stderr)
		outStream = stdcopy.NewStdWriter(outStream, stdcopy.Stdout)
	}

	var stdin io.ReadCloser
	var stdout, stderr io.Writer

	if c.UseStdin {
		stdin = inStream
	}
	if c.UseStdout {
		stdout = outStream
	}
	if c.UseStderr {
		stderr = errStream
	}

	if err := daemon.containerAttach(container, stdin, stdout, stderr, c.Logs, c.Stream, c.DetachKeys); err != nil {
		fmt.Fprintf(outStream, "Error attaching: %s\n", err)
	}
	return nil
}

// ContainerAttachRaw attaches the provided streams to the container's stdio
func (daemon *Daemon) ContainerAttachRaw(prefixOrName string, stdin io.ReadCloser, stdout, stderr io.Writer, stream bool) error {
	container, err := daemon.GetContainer(prefixOrName)
	if err != nil {
		return err
	}
	return daemon.containerAttach(container, stdin, stdout, stderr, false, stream, nil)
}

func (daemon *Daemon) containerAttach(container *container.Container, stdin io.ReadCloser, stdout, stderr io.Writer, logs, stream bool, keys []byte) error {
	if logs {
		logDriver, err := daemon.getLogger(container)
		if err != nil {
			return err
		}
		cLog, ok := logDriver.(logger.LogReader)
		if !ok {
			return logger.ErrReadLogsNotSupported
		}
		logs := cLog.ReadLogs(logger.ReadConfig{Tail: -1})

	LogLoop:
		for {
			select {
			case msg, ok := <-logs.Msg:
				if !ok {
					break LogLoop
				}
				if msg.Source == "stdout" && stdout != nil {
					stdout.Write(msg.Line)
				}
				if msg.Source == "stderr" && stderr != nil {
					stderr.Write(msg.Line)
				}
			case err := <-logs.Err:
				logrus.Errorf("Error streaming logs: %v", err)
				break LogLoop
			}
		}
	}

	daemon.LogContainerEvent(container, "attach")

	//stream
	if stream {
		var stdinPipe io.ReadCloser
		if stdin != nil {
			r, w := io.Pipe()
			go func() {
				defer w.Close()
				defer logrus.Debugf("Closing buffered stdin pipe")
				io.Copy(w, stdin)
			}()
			stdinPipe = r
		}
		<-container.Attach(stdinPipe, stdout, stderr, keys)
		// If we are in stdinonce mode, wait for the process to end
		// otherwise, simply return
		if container.Config.StdinOnce && !container.Config.Tty {
			container.WaitStop(-1 * time.Second)
		}
	}
	return nil
}
