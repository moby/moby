package daemon

import (
	"context"
	"fmt"
	"io"

	"github.com/containerd/log"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client/pkg/stdcopy"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/internal/stdcopymux"
	"github.com/moby/moby/v2/daemon/internal/stream"
	"github.com/moby/moby/v2/daemon/logger"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/errdefs"
	"github.com/moby/term"
	"github.com/pkg/errors"
)

// ContainerAttach attaches to logs according to the config passed in. See ContainerAttachConfig.
func (daemon *Daemon) ContainerAttach(prefixOrName string, req *backend.ContainerAttachConfig) error {
	keys := []byte{}
	var err error
	if req.DetachKeys != "" {
		keys, err = term.ToBytes(req.DetachKeys)
		if err != nil {
			return errdefs.InvalidParameter(errors.Errorf("Invalid detach keys (%s) provided", req.DetachKeys))
		}
	}

	ctr, err := daemon.GetContainer(prefixOrName)
	if err != nil {
		return err
	}
	if ctr.State.IsPaused() {
		return errdefs.Conflict(fmt.Errorf("container %s is paused, unpause the container before attach", prefixOrName))
	}
	if ctr.State.IsRestarting() {
		return errdefs.Conflict(fmt.Errorf("container %s is restarting, wait until the container is running", prefixOrName))
	}

	cfg := stream.AttachConfig{
		UseStdin:   req.UseStdin,
		UseStdout:  req.UseStdout,
		UseStderr:  req.UseStderr,
		TTY:        ctr.Config.Tty,
		CloseStdin: ctr.Config.StdinOnce,
		DetachKeys: keys,
	}
	ctr.StreamConfig.AttachStreams(&cfg)

	clientCtx, closeNotify := context.WithCancel(context.Background())
	defer closeNotify()
	go func() {
		<-clientCtx.Done()
		// The client has disconnected
		// In this case we need to close the container's output streams so that the goroutines used to copy
		// to the client streams are unblocked and can exit.
		if cfg.CStdout != nil {
			cfg.CStdout.Close()
		}
		if cfg.CStderr != nil {
			cfg.CStderr.Close()
		}
	}()

	multiplexed := !ctr.Config.Tty && req.MuxStreams
	inStream, outStream, errStream, err := req.GetStreams(multiplexed, closeNotify)
	if err != nil {
		return err
	}

	defer inStream.Close()

	if multiplexed {
		errStream = stdcopymux.NewStdWriter(errStream, stdcopy.Stderr)
		outStream = stdcopymux.NewStdWriter(outStream, stdcopy.Stdout)
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

	if err := daemon.containerAttach(ctr, &cfg, req.Logs, req.Stream); err != nil {
		_, _ = fmt.Fprintln(outStream, "Error attaching:", err)
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

func (daemon *Daemon) containerAttach(ctr *container.Container, cfg *stream.AttachConfig, enableLogs, doStream bool) error {
	if enableLogs {
		logDriver, logCreated, err := daemon.getLogger(ctr)
		if err != nil {
			return err
		}
		if logCreated {
			defer func() {
				if err = logDriver.Close(); err != nil {
					log.G(context.TODO()).WithFields(log.Fields{
						"error":     err,
						"container": ctr.ID,
					}).Error("Error closing logger")
				}
			}()
		}
		cLog, ok := logDriver.(logger.LogReader)
		if !ok {
			return logger.ErrReadLogsNotSupported{}
		}
		logWatcher := cLog.ReadLogs(context.TODO(), logger.ReadConfig{Tail: -1})
		defer logWatcher.ConsumerGone()

	LogLoop:
		for {
			select {
			case msg, ok := <-logWatcher.Msg:
				if !ok {
					break LogLoop
				}
				if msg.Source == "stdout" && cfg.Stdout != nil {
					cfg.Stdout.Write(msg.Line)
				}
				if msg.Source == "stderr" && cfg.Stderr != nil {
					cfg.Stderr.Write(msg.Line)
				}
			case err := <-logWatcher.Err:
				log.G(context.TODO()).WithFields(log.Fields{
					"error":     err,
					"container": ctr.ID,
				}).Error("Error streaming logs")
				break LogLoop
			}
		}
	}

	daemon.LogContainerEvent(ctr, events.ActionAttach)

	if !doStream {
		return nil
	}

	if cfg.Stdin != nil {
		r, w := io.Pipe()
		go func(stdin io.ReadCloser) {
			io.Copy(w, stdin)
			log.G(context.TODO()).WithFields(log.Fields{
				"container": ctr.ID,
			}).Debug("Closing buffered stdin pipe")
			w.Close()
		}(cfg.Stdin)
		cfg.Stdin = r
	}

	if !ctr.Config.OpenStdin {
		cfg.Stdin = nil
	}

	if ctr.Config.StdinOnce && !ctr.Config.Tty {
		// Wait for the container to stop before returning.
		waitChan := ctr.State.Wait(context.Background(), containertypes.WaitConditionNotRunning)
		defer func() {
			<-waitChan // Ignore returned exit code.
		}()
	}

	ctx := ctr.AttachContext()
	err := <-ctr.StreamConfig.CopyStreams(ctx, cfg)
	if err != nil {
		var ierr term.EscapeError
		if errors.Is(err, context.Canceled) || errors.As(err, &ierr) {
			daemon.LogContainerEvent(ctr, events.ActionDetach)
		} else {
			log.G(ctx).WithFields(log.Fields{
				"error":     err,
				"container": ctr.ID,
			}).Error("attach failed with error")
		}
	}

	return nil
}
