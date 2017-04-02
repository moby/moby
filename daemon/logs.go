package daemon

import (
	"errors"
	"io"
	"strconv"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/backend"
	containertypes "github.com/docker/docker/api/types/container"
	timetypes "github.com/docker/docker/api/types/time"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/stdcopy"
)

// ContainerLogsRawChan, given a channel, returns raw log message structs from
// the container logger.
func (daemon *Daemon) ContainerLogsRawChan(ctx context.Context, containerName string, config *backend.ContainerLogsConfig, msgs chan *logger.Message) error {
	// TODO(dperny) there might be something weird with returning log messages
	// straight off that channel. there's some kind of ring buffer log thing
	// available so i don't actually know how long the pointers i'm returning
	// are valid for. watch out for weird errors about it

	// close the messages channel
	defer close(msgs)
	if !(config.ShowStdout || config.ShowStderr) {
		return errors.New("You must choose at least one stream")
	}
	container, err := daemon.GetContainer(containerName)
	if err != nil {
		return err
	}

	if container.HostConfig.LogConfig.Type == "none" {
		return logger.ErrReadLogsNotSupported
	}

	cLog, err := daemon.getLogger(container)
	if err != nil {
		return err
	}

	logReader, ok := cLog.(logger.LogReader)
	if !ok {
		return logger.ErrReadLogsNotSupported
	}

	follow := config.Follow && container.IsRunning()
	tailLines, err := strconv.Atoi(config.Tail)
	if err != nil {
		tailLines = -1
	}

	logrus.Debug("logs: begin stream")

	var since time.Time
	if config.Since != "" {
		s, n, err := timetypes.ParseTimestamps(config.Since, 0)
		if err != nil {
			return err
		}
		since = time.Unix(s, n)
	}

	readConfig := logger.ReadConfig{
		Since:  since,
		Tail:   tailLines,
		Follow: follow,
	}

	logs := logReader.ReadLogs(readConfig)
	defer func() {
		logs.Close()
		if cLog != container.LogDriver {
			// Since the logger isn't cached in the container, which
			// occurs if it is running, it must get explicitly closed
			// here to avoid leaking it and any file handles it has.
			if err := cLog.Close(); err != nil {
				logrus.Errorf("Error closing logger: %v", err)
			}
		}
	}()

	// TODO(dperny) fix logrus to be pretty
	for {
		select {
		case err := <-logs.Err:
			logrus.Errorf("Error streaming logs: %v", err)
			return nil
		case <-ctx.Done():
			logrus.Debug("logs: end stream, ctx is done: %v", ctx.Err())
			return nil
		case msg, ok := <-logs.Msg:
			if !ok {
				logrus.Debug("logs: end stream")
				return nil
			}
			// TODO(dperny) this will block here if msgs isn't buffered
			msgs <- msg
		}
	}
}

// ContainerLogs hooks up a container's stdout and stderr streams
// configured with the given struct.
func (daemon *Daemon) ContainerLogs(ctx context.Context, containerName string, config *backend.ContainerLogsConfig, started chan struct{}) error {
	chMsgs := make(chan *logger.Message, 1)

	container, err := daemon.GetContainer(containerName)
	if err != nil {
		return err
	}

	wf := ioutils.NewWriteFlusher(config.OutStream)
	defer wf.Close()
	close(started)
	wf.Flush()

	var outStream io.Writer
	outStream = wf
	errStream := outStream
	if !container.Config.Tty {
		errStream = stdcopy.NewStdWriter(outStream, stdcopy.Stderr)
		outStream = stdcopy.NewStdWriter(outStream, stdcopy.Stdout)
	}

	// gerr belongs to this gofunc
	// TODO(dperny) sloppy and racy
	gerr := make(chan error)
	go func() {
		gerr <- daemon.ContainerLogsRawChan(ctx, containerName, config, chMsgs)
	}()

	for {
		select {
		case err := <-gerr:
			return err
		case <-ctx.Done():
			logrus.Debugf("logs: end stream, ctx is done: %v", ctx.Err())
			return nil
		case msg, ok := <-chMsgs:
			if !ok {
				logrus.Debug("logs: end stream")
				return nil
			}
			logLine := msg.Line
			if config.Details {
				logLine = append([]byte(msg.Attrs.String()+" "), logLine...)
			}
			if config.Timestamps {
				logLine = append([]byte(msg.Timestamp.Format(logger.TimeFormat)+" "), logLine...)
			}
			if msg.Source == "stdout" && config.ShowStdout {
				outStream.Write(logLine)
			}
			if msg.Source == "stderr" && config.ShowStderr {
				errStream.Write(logLine)
			}
		}
	}

}
func (daemon *Daemon) getLogger(container *container.Container) (logger.Logger, error) {
	if container.LogDriver != nil && container.IsRunning() {
		return container.LogDriver, nil
	}
	return container.StartLogger()
}

// mergeLogConfig merges the daemon log config to the container's log config if the container's log driver is not specified.
func (daemon *Daemon) mergeAndVerifyLogConfig(cfg *containertypes.LogConfig) error {
	if cfg.Type == "" {
		cfg.Type = daemon.defaultLogConfig.Type
	}

	if cfg.Config == nil {
		cfg.Config = make(map[string]string)
	}

	if cfg.Type == daemon.defaultLogConfig.Type {
		for k, v := range daemon.defaultLogConfig.Config {
			if _, ok := cfg.Config[k]; !ok {
				cfg.Config[k] = v
			}
		}
	}

	return logger.ValidateLogOpts(cfg.Type, cfg.Config)
}
