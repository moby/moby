package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/container"
	"github.com/docker/docker/container/stream"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/moby/term"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ContainerAttach attaches to logs according to the config passed in. See ContainerAttachConfig.
func (daemon *Daemon) ContainerAttach(prefixOrName string, c *backend.ContainerAttachConfig) error {
	keys := []byte{}
	var err error
	if c.DetachKeys != "" {
		keys, err = term.ToBytes(c.DetachKeys)
		if err != nil {
			return errdefs.InvalidParameter(errors.Errorf("Invalid detach keys (%s) provided", c.DetachKeys))
		}
	}

	ctr, err := daemon.GetContainer(prefixOrName)
	if err != nil {
		return err
	}
	if ctr.IsPaused() {
		err := fmt.Errorf("container %s is paused, unpause the container before attach", prefixOrName)
		return errdefs.Conflict(err)
	}
	if ctr.IsRestarting() {
		err := fmt.Errorf("container %s is restarting, wait until the container is running", prefixOrName)
		return errdefs.Conflict(err)
	}

	cfg := stream.AttachConfig{
		UseStdin:   c.UseStdin,
		UseStdout:  c.UseStdout,
		UseStderr:  c.UseStderr,
		TTY:        ctr.Config.Tty,
		CloseStdin: ctr.Config.StdinOnce,
		DetachKeys: keys,
	}
	ctr.StreamConfig.AttachStreams(&cfg)

	multiplexed := !ctr.Config.Tty && c.MuxStreams
	inStream, outStream, errStream, err := c.GetStreams(multiplexed)
	if err != nil {
		return err
	}
	defer inStream.Close()

	if multiplexed {
		errStream = stdcopy.NewStdWriter(errStream, stdcopy.Stderr)
		outStream = stdcopy.NewStdWriter(outStream, stdcopy.Stdout)
	}

	if cfg.UseStdin {
		cfg.Stdin = inStream
	}
	if cfg.UseStdout {
		cfg.Stdout = outStream
	}
	if cfg.UseStderr {
		cfg.Stderr = errStream
	}

	if err := daemon.containerAttach(ctr, &cfg, c.Logs, c.Stream); err != nil {
		fmt.Fprintf(outStream, "Error attaching: %s\n", err)
	}
	return nil
}

// ContainerAttachRaw attaches the provided streams to the container's stdio
func (daemon *Daemon) ContainerAttachRaw(prefixOrName string, stdin io.ReadCloser, stdout, stderr io.Writer, doStream bool, attached chan struct{}) error {
	ctr, err := daemon.GetContainer(prefixOrName)
	if err != nil {
		return err
	}
	cfg := stream.AttachConfig{
		UseStdin:   stdin != nil,
		UseStdout:  stdout != nil,
		UseStderr:  stderr != nil,
		TTY:        ctr.Config.Tty,
		CloseStdin: ctr.Config.StdinOnce,
	}
	ctr.StreamConfig.AttachStreams(&cfg)
	close(attached)
	if cfg.UseStdin {
		cfg.Stdin = stdin
	}
	if cfg.UseStdout {
		cfg.Stdout = stdout
	}
	if cfg.UseStderr {
		cfg.Stderr = stderr
	}

	return daemon.containerAttach(ctr, &cfg, false, doStream)
}

func (daemon *Daemon) containerAttach(c *container.Container, cfg *stream.AttachConfig, logs, doStream bool) error {
	if logs {
		logDriver, logCreated, err := daemon.getLogger(c)
		if err != nil {
			return err
		}
		if logCreated {
			defer func() {
				if err = logDriver.Close(); err != nil {
					logrus.Errorf("Error closing logger: %v", err)
				}
			}()
		}
		cLog, ok := logDriver.(logger.LogReader)
		if !ok {
			return logger.ErrReadLogsNotSupported{}
		}
		logs := cLog.ReadLogs(logger.ReadConfig{Tail: -1})
		defer logs.ConsumerGone()

	LogLoop:
		for {
			select {
			case msg, ok := <-logs.Msg:
				if !ok {
					break LogLoop
				}
				if msg.Source == "stdout" && cfg.Stdout != nil {
					cfg.Stdout.Write(msg.Line)
				}
				if msg.Source == "stderr" && cfg.Stderr != nil {
					cfg.Stderr.Write(msg.Line)
				}
			case err := <-logs.Err:
				logrus.Errorf("Error streaming logs: %v", err)
				break LogLoop
			}
		}
	}

	daemon.LogContainerEvent(c, "attach")

	if !doStream {
		return nil
	}

	if cfg.Stdin != nil {
		r, w := io.Pipe()
		go func(stdin io.ReadCloser) {
			defer w.Close()
			defer logrus.Debug("Closing buffered stdin pipe")
			io.Copy(w, stdin)
		}(cfg.Stdin)
		cfg.Stdin = r
	}

	if !c.Config.OpenStdin {
		cfg.Stdin = nil
	}

	if c.Config.StdinOnce && !c.Config.Tty {
		// Wait for the container to stop before returning.
		waitChan := c.Wait(context.Background(), container.WaitConditionNotRunning)
		defer func() {
			<-waitChan // Ignore returned exit code.
		}()
	}

	ctx := c.InitAttachContext()
	err := <-c.StreamConfig.CopyStreams(ctx, cfg)
	if err != nil {
		var ierr term.EscapeError
		if errors.Is(err, context.Canceled) || errors.As(err, &ierr) {
			daemon.LogContainerEvent(c, "detach")
		} else {
			logrus.Errorf("attach failed with error: %v", err)
		}
	}

	return nil
}
