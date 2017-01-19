package daemon

import (
	"fmt"
	"io"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/errors"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/container"
	"github.com/docker/docker/container/stream"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
)

type containerAttachConfig struct {
	detachKeys     []byte
	stdin          io.ReadCloser
	stdout, stderr io.Writer
	showHistory    bool
	stream         bool
}

// ContainerAttach attaches to logs according to the config passed in. See ContainerAttachConfig.
func (daemon *Daemon) ContainerAttach(prefixOrName string, c *backend.ContainerAttachConfig) error {
	keys := []byte{}
	var err error
	if c.DetachKeys != "" {
		keys, err = term.ToBytes(c.DetachKeys)
		if err != nil {
			return fmt.Errorf("Invalid escape keys (%s) provided", c.DetachKeys)
		}
	}

	container, err := daemon.GetContainer(prefixOrName)
	if err != nil {
		return err
	}
	if container.IsPaused() {
		err := fmt.Errorf("Container %s is paused. Unpause the container before attach", prefixOrName)
		return errors.NewRequestConflictError(err)
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

	var cfg containerAttachConfig

	if c.UseStdin {
		cfg.stdin = inStream
	}
	if c.UseStdout {
		cfg.stdout = outStream
	}
	if c.UseStderr {
		cfg.stderr = errStream
	}

	cfg.showHistory = c.Logs
	cfg.stream = c.Stream
	cfg.detachKeys = keys

	if err := daemon.containerAttach(container, &cfg); err != nil {
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
	cfg := &containerAttachConfig{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		stream: stream,
	}
	return daemon.containerAttach(container, cfg)
}

func (daemon *Daemon) containerAttach(c *container.Container, cfg *containerAttachConfig) error {
	stdin := cfg.stdin
	stdout := cfg.stdout
	stderr := cfg.stderr
	if cfg.showHistory {
		logDriver, err := daemon.getLogger(c)
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

	daemon.LogContainerEvent(c, "attach")

	if !cfg.stream {
		return nil
	}

	var stdinPipe io.ReadCloser
	if stdin != nil {
		r, w := io.Pipe()
		go func() {
			defer w.Close()
			defer logrus.Debug("Closing buffered stdin pipe")
			io.Copy(w, stdin)
		}()
		stdinPipe = r
	}

	waitChan := make(chan struct{})
	if c.Config.StdinOnce && !c.Config.Tty {
		defer func() {
			<-waitChan
		}()
		go func() {
			c.WaitStop(-1 * time.Second)
			close(waitChan)
		}()
	}

	aCfg := &stream.AttachConfig{
		Stdin:      stdinPipe,
		Stdout:     stdout,
		Stderr:     stderr,
		DetachKeys: cfg.detachKeys,
	}

	err := <-c.Attach(aCfg)
	if err != nil {
		if _, ok := err.(stream.DetachError); ok {
			daemon.LogContainerEvent(c, "detach")
		} else {
			logrus.Errorf("attach failed with error: %v", err)
		}
	}

	return nil
}
