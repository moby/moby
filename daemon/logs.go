package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/tailfile"
	"github.com/docker/docker/pkg/timeutils"
)

type ContainerLogsConfig struct {
	Follow, Timestamps   bool
	Tail                 string
	UseStdout, UseStderr bool
	OutStream            io.Writer
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

	if container.LogDriverType() != "json-file" {
		return fmt.Errorf("\"logs\" endpoint is supported only for \"json-file\" logging driver")
	}
	cLog, err := container.ReadLog("json")
	if err != nil && os.IsNotExist(err) {
		// Legacy logs
		logrus.Debugf("Old logs format")
		if config.UseStdout {
			cLog, err := container.ReadLog("stdout")
			if err != nil {
				logrus.Errorf("Error reading logs (stdout): %s", err)
			} else if _, err := io.Copy(outStream, cLog); err != nil {
				logrus.Errorf("Error streaming logs (stdout): %s", err)
			}
		}
		if config.UseStderr {
			cLog, err := container.ReadLog("stderr")
			if err != nil {
				logrus.Errorf("Error reading logs (stderr): %s", err)
			} else if _, err := io.Copy(errStream, cLog); err != nil {
				logrus.Errorf("Error streaming logs (stderr): %s", err)
			}
		}
	} else if err != nil {
		logrus.Errorf("Error reading logs (json): %s", err)
	} else {
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
				if err := dec.Decode(l); err == io.EOF {
					break
				} else if err != nil {
					logrus.Errorf("Error streaming logs: %s", err)
					break
				}
				logLine := l.Log
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
				l.Reset()
			}
		}
	}
	if config.Follow && container.IsRunning() {
		errors := make(chan error, 2)
		wg := sync.WaitGroup{}

		// write an empty chunk of data (this is to ensure that the
		// HTTP Response is sent immediatly, even if the container has
		// not yet produced any data)
		outStream.Write(nil)

		if config.UseStdout {
			wg.Add(1)
			stdoutPipe := container.StdoutLogPipe()
			defer stdoutPipe.Close()
			go func() {
				errors <- jsonlog.WriteLog(stdoutPipe, outStream, format)
				wg.Done()
			}()
		}
		if config.UseStderr {
			wg.Add(1)
			stderrPipe := container.StderrLogPipe()
			defer stderrPipe.Close()
			go func() {
				errors <- jsonlog.WriteLog(stderrPipe, errStream, format)
				wg.Done()
			}()
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			if err != nil {
				logrus.Errorf("%s", err)
			}
		}

	}
	return nil
}
