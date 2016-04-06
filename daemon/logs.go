package daemon

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/jsonfilelog"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/stdcopy"
	containertypes "github.com/docker/engine-api/types/container"
	timetypes "github.com/docker/engine-api/types/time"
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

	var outStream io.Writer = wf
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
				logrus.Debugf("logs: end stream")
				logs.Close()
				return nil
			}
			logLine := msg.Line
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
	cfg := daemon.getLogConfig(container.HostConfig.LogConfig)
	if err := logger.ValidateLogOpts(cfg.Type, cfg.Config); err != nil {
		return nil, err
	}
	return container.StartLogger(cfg)
}

// StartLogging initializes and starts the container logging stream.
func (daemon *Daemon) StartLogging(container *container.Container) error {
	cfg := daemon.getLogConfig(container.HostConfig.LogConfig)
	if cfg.Type == "none" {
		return nil // do not start logging routines
	}

	if err := logger.ValidateLogOpts(cfg.Type, cfg.Config); err != nil {
		return err
	}
	l, err := container.StartLogger(cfg)
	if err != nil {
		return fmt.Errorf("Failed to initialize logging driver: %v", err)
	}

	copier := logger.NewCopier(container.ID, map[string]io.Reader{"stdout": container.StdoutPipe(), "stderr": container.StderrPipe()}, l)
	container.LogCopier = copier
	copier.Run()
	container.LogDriver = l

	// set LogPath field only for json-file logdriver
	if jl, ok := l.(*jsonfilelog.JSONFileLogger); ok {
		container.LogPath = jl.LogPath()
	}

	return nil
}

// getLogConfig returns the log configuration for the container.
func (daemon *Daemon) getLogConfig(cfg containertypes.LogConfig) containertypes.LogConfig {
	if cfg.Type != "" || len(cfg.Config) > 0 { // container has log driver configured
		if cfg.Type == "" {
			cfg.Type = jsonfilelog.Name
		}
		return cfg
	}

	// Use daemon's default log config for containers
	return daemon.defaultLogConfig
}
