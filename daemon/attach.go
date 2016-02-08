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

// ContainerAttachWithLogs attaches to logs according to the config passed in. See ContainerAttachWithLogsConfig.
func (daemon *Daemon) ContainerAttachWithLogs(prefixOrName string, c *backend.ContainerAttachWithLogsConfig) error {
	if c.Hijacker == nil {
		return derr.ErrorCodeNoHijackConnection.WithArgs(prefixOrName)
	}
	container, err := daemon.GetContainer(prefixOrName)
	if err != nil {
		return derr.ErrorCodeNoSuchContainer.WithArgs(prefixOrName)
	}
	if container.IsPaused() {
		return derr.ErrorCodePausedContainer.WithArgs(prefixOrName)
	}

	conn, _, err := c.Hijacker.Hijack()
	if err != nil {
		return err
	}
	defer conn.Close()
	// Flush the options to make sure the client sets the raw mode
	conn.Write([]byte{})
	inStream := conn.(io.ReadCloser)
	outStream := conn.(io.Writer)

	if c.Upgrade {
		fmt.Fprintf(outStream, "HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
	} else {
		fmt.Fprintf(outStream, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	}

	var errStream io.Writer

	if !container.Config.Tty {
		errStream = stdcopy.NewStdWriter(outStream, stdcopy.Stderr)
		outStream = stdcopy.NewStdWriter(outStream, stdcopy.Stdout)
	} else {
		errStream = outStream
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

	if err := daemon.attachWithLogs(container, stdin, stdout, stderr, c.Logs, c.Stream, c.DetachKeys); err != nil {
		fmt.Fprintf(outStream, "Error attaching: %s\n", err)
	}
	return nil
}

// ContainerWsAttachWithLogs websocket connection
func (daemon *Daemon) ContainerWsAttachWithLogs(prefixOrName string, c *backend.ContainerWsAttachWithLogsConfig) error {
	container, err := daemon.GetContainer(prefixOrName)
	if err != nil {
		return err
	}
	return daemon.attachWithLogs(container, c.InStream, c.OutStream, c.ErrStream, c.Logs, c.Stream, c.DetachKeys)
}

// ContainerAttachOnBuild attaches streams to the container cID. If stream is true, it streams the output.
func (daemon *Daemon) ContainerAttachOnBuild(cID string, stdin io.ReadCloser, stdout, stderr io.Writer, stream bool) error {
	return daemon.ContainerWsAttachWithLogs(cID, &backend.ContainerWsAttachWithLogsConfig{
		InStream:  stdin,
		OutStream: stdout,
		ErrStream: stderr,
		Stream:    stream,
	})
}

func (daemon *Daemon) attachWithLogs(container *container.Container, stdin io.ReadCloser, stdout, stderr io.Writer, logs, stream bool, keys []byte) error {
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
