package daemon

import (
	"fmt"
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
	"github.com/docker/docker/daemon/logger/jsonfilelog"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/stdcopy"
)

// ContainerLogs hooks up a container's stdout and stderr streams
// configured with the given struct.
func (daemon *Daemon) ContainerLogs(ctx context.Context, containerName string, config *backend.ContainerLogsConfig, started chan struct{}) error {
	container, err := daemon.GetContainer(containerName)
	if err != nil {
		return err
	}

	if !(config.ShowStdout || config.ShowStderr) {
		return fmt.Errorf("You must choose at least one stream")
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

	for {
		select {
		case err := <-logs.Err:
			logrus.Errorf("Error streaming logs: %v", err)
			return nil
		case <-ctx.Done():
			logs.Close()
			return nil
		case msg, ok := <-logs.Msg:
			if !ok {
				logrus.Debug("logs: end stream")
				logs.Close()
				if cLog != container.LogDriver {
					// Since the logger isn't cached in the container, which occurs if it is running, it
					// must get explicitly closed here to avoid leaking it and any file handles it has.
					if err := cLog.Close(); err != nil {
						logrus.Errorf("Error closing logger: %v", err)
					}
				}
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
	return container.StartLogger(container.HostConfig.LogConfig)
}

// StartLogging initializes and starts the container logging stream.
func (daemon *Daemon) StartLogging(container *container.Container) error {
	if container.HostConfig.LogConfig.Type == "none" {
		return nil // do not start logging routines
	}

	l, err := container.StartLogger(container.HostConfig.LogConfig)
	if err != nil {
		return fmt.Errorf("Failed to initialize logging driver: %v", err)
	}

	copier := logger.NewCopier(map[string]io.Reader{"stdout": container.StdoutPipe(), "stderr": container.StderrPipe()}, l)
	container.LogCopier = copier
	copier.Run()
	container.LogDriver = l

	// set LogPath field only for json-file logdriver
	if jl, ok := l.(*jsonfilelog.JSONFileLogger); ok {
		container.LogPath = jl.LogPath()
	}

	return nil
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
