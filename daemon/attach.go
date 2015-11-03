package daemon

import (
	"io"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/stdcopy"
)

// ContainerAttachWithLogsConfig holds the streams to use when connecting to a container to view logs.
type ContainerAttachWithLogsConfig struct {
	InStream                       io.ReadCloser
	OutStream                      io.Writer
	UseStdin, UseStdout, UseStderr bool
	Logs, Stream                   bool
}

// ContainerAttachWithLogs attaches to logs according to the config passed in. See ContainerAttachWithLogsConfig.
func (daemon *Daemon) ContainerAttachWithLogs(prefixOrName string, c *ContainerAttachWithLogsConfig) error {
	container, err := daemon.Get(prefixOrName)
	if err != nil {
		return err
	}

	var errStream io.Writer

	if !container.Config.Tty {
		errStream = stdcopy.NewStdWriter(c.OutStream, stdcopy.Stderr)
		c.OutStream = stdcopy.NewStdWriter(c.OutStream, stdcopy.Stdout)
	} else {
		errStream = c.OutStream
	}

	var stdin io.ReadCloser
	var stdout, stderr io.Writer

	if c.UseStdin {
		stdin = c.InStream
	}
	if c.UseStdout {
		stdout = c.OutStream
	}
	if c.UseStderr {
		stderr = errStream
	}

	return daemon.attachWithLogs(container, stdin, stdout, stderr, c.Logs, c.Stream)
}

// ContainerWsAttachWithLogsConfig attach with websockets, since all
// stream data is delegated to the websocket to handle there.
type ContainerWsAttachWithLogsConfig struct {
	InStream             io.ReadCloser
	OutStream, ErrStream io.Writer
	Logs, Stream         bool
}

// ContainerWsAttachWithLogs websocket connection
func (daemon *Daemon) ContainerWsAttachWithLogs(prefixOrName string, c *ContainerWsAttachWithLogsConfig) error {
	container, err := daemon.Get(prefixOrName)
	if err != nil {
		return err
	}
	return daemon.attachWithLogs(container, c.InStream, c.OutStream, c.ErrStream, c.Logs, c.Stream)
}

func (daemon *Daemon) attachWithLogs(container *Container, stdin io.ReadCloser, stdout, stderr io.Writer, logs, stream bool) error {
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
		<-container.Attach(stdinPipe, stdout, stderr)
		// If we are in stdinonce mode, wait for the process to end
		// otherwise, simply return
		if container.Config.StdinOnce && !container.Config.Tty {
			container.WaitStop(-1 * time.Second)
		}
	}
	return nil
}
