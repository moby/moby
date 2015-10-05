package daemon

import (
	"io"

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

	return container.attachWithLogs(stdin, stdout, stderr, c.Logs, c.Stream)
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
	return container.attachWithLogs(c.InStream, c.OutStream, c.ErrStream, c.Logs, c.Stream)
}
