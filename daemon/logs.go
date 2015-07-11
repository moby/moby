package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger/jsonfilelog"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/tailfile"
	"github.com/docker/docker/pkg/timeutils"
)

type ContainerLogsConfig struct {
	Follow, Timestamps   bool
	Tail                 string
	Since                time.Time
	UseStdout, UseStderr bool
	OutStream            io.Writer
	Stop                 <-chan bool
}

func (daemon *Daemon) ContainerLogs(name string, config *ContainerLogsConfig) error {
	var (
		lines  = -1
		format string
	)
	if !(config.UseStdout || config.UseStderr) {
		return fmt.Errorf("You must choose at least one stream")
	}
	if config.Timestamps {
		format = timeutils.RFC3339NanoFixed
	}
	if config.Tail == "" {
		config.Tail = "all"
	}

	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	var (
		outStream = config.OutStream
		errStream io.Writer
	)
	if !container.Config.Tty {
		errStream = stdcopy.NewStdWriter(outStream, stdcopy.Stderr)
		outStream = stdcopy.NewStdWriter(outStream, stdcopy.Stdout)
	} else {
		errStream = outStream
	}

	if container.LogDriverType() != jsonfilelog.Name {
		return fmt.Errorf("\"logs\" endpoint is supported only for \"json-file\" logging driver")
	}
	logDriver, err := container.getLogger()
	cLog, err := logDriver.GetReader()
	if err != nil {
		logrus.Errorf("Error reading logs: %s", err)
	} else {
		// json-file driver
		if config.Tail != "all" {
			var err error
			lines, err = strconv.Atoi(config.Tail)
			if err != nil {
				logrus.Errorf("Failed to parse tail %s, error: %v, show all logs", config.Tail, err)
				lines = -1
			}
		}

		if lines != 0 {
			if lines > 0 {
				f := cLog.(*os.File)
				ls, err := tailfile.TailFile(f, lines)
				if err != nil {
					return err
				}
				tmp := bytes.NewBuffer([]byte{})
				for _, l := range ls {
					fmt.Fprintf(tmp, "%s\n", l)
				}
				cLog = tmp
			}

			dec := json.NewDecoder(cLog)
			l := &jsonlog.JSONLog{}
			for {
				l.Reset()
				if err := dec.Decode(l); err == io.EOF {
					break
				} else if err != nil {
					logrus.Errorf("Error streaming logs: %s", err)
					break
				}
				logLine := l.Log
				if !config.Since.IsZero() && l.Created.Before(config.Since) {
					continue
				}
				if config.Timestamps {
					// format can be "" or time format, so here can't be error
					logLine, _ = l.Format(format)
				}
				if l.Stream == "stdout" && config.UseStdout {
					io.WriteString(outStream, logLine)
				}
				if l.Stream == "stderr" && config.UseStderr {
					io.WriteString(errStream, logLine)
				}
			}
		}
	}

	if config.Follow && container.IsRunning() {
		chErrStderr := make(chan error)
		chErrStdout := make(chan error)
		var stdoutPipe, stderrPipe io.ReadCloser

		// write an empty chunk of data (this is to ensure that the
		// HTTP Response is sent immediatly, even if the container has
		// not yet produced any data)
		outStream.Write(nil)

		if config.UseStdout {
			stdoutPipe = container.StdoutLogPipe()
			go func() {
				logrus.Debug("logs: stdout stream begin")
				chErrStdout <- jsonlog.WriteLog(stdoutPipe, outStream, format, config.Since)
				logrus.Debug("logs: stdout stream end")
			}()
		}
		if config.UseStderr {
			stderrPipe = container.StderrLogPipe()
			go func() {
				logrus.Debug("logs: stderr stream begin")
				chErrStderr <- jsonlog.WriteLog(stderrPipe, errStream, format, config.Since)
				logrus.Debug("logs: stderr stream end")
			}()
		}

		select {
		case err = <-chErrStderr:
			if stdoutPipe != nil {
				stdoutPipe.Close()
				<-chErrStdout
			}
		case err = <-chErrStdout:
			if stderrPipe != nil {
				stderrPipe.Close()
				<-chErrStderr
			}
		case <-config.Stop:
			if stdoutPipe != nil {
				stdoutPipe.Close()
				<-chErrStdout
			}
			if stderrPipe != nil {
				stderrPipe.Close()
				<-chErrStderr
			}
			return nil
		}

		if err != nil && err != io.EOF && err != io.ErrClosedPipe {
			if e, ok := err.(*net.OpError); ok && e.Err != syscall.EPIPE {
				logrus.Errorf("error streaming logs: %v", err)
			}
		}
	}
	return nil
}
