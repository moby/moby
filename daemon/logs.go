package daemon

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/stdcopy"
)

type ContainerLogsConfig struct {
	Follow, Timestamps   bool
	Tail                 string
	Since                time.Time
	UseStdout, UseStderr bool
	OutStream            io.Writer
	Stop                 <-chan bool
}

func (daemon *Daemon) ContainerLogs(container *Container, config *ContainerLogsConfig) error {
	if !(config.UseStdout || config.UseStderr) {
		return fmt.Errorf("You must choose at least one stream")
	}

	outStream := config.OutStream
	errStream := outStream
	if !container.Config.Tty {
		errStream = stdcopy.NewStdWriter(outStream, stdcopy.Stderr)
		outStream = stdcopy.NewStdWriter(outStream, stdcopy.Stdout)
	}

	cLog, err := container.getLogger()
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
	readConfig := logger.ReadConfig{
		Since:  config.Since,
		Tail:   tailLines,
		Follow: follow,
	}
	logs := logReader.ReadLogs(readConfig)

	for {
		select {
		case err := <-logs.Err:
			logrus.Errorf("Error streaming logs: %v", err)
			return nil
		case <-config.Stop:
			logs.Close()
			return nil
		case msg, ok := <-logs.Msg:
			if !ok {
				logrus.Debugf("logs: end stream")
				return nil
			}
			logLine := msg.Line
			if config.Timestamps {
				logLine = append([]byte(msg.Timestamp.Format(logger.TimeFormat)+" "), logLine...)
			}
			if msg.Source == "stdout" && config.UseStdout {
				outStream.Write(logLine)
			}
			if msg.Source == "stderr" && config.UseStderr {
				errStream.Write(logLine)
			}
		}
	}
}
