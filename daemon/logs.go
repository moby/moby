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
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/tailfile"
	"github.com/docker/docker/pkg/timeutils"
)

func (daemon *Daemon) ContainerLogs(job *engine.Job) error {
	if len(job.Args) != 1 {
		return fmt.Errorf("Usage: %s CONTAINER\n", job.Name)
	}

	var (
		name   = job.Args[0]
		stdout = job.GetenvBool("stdout")
		stderr = job.GetenvBool("stderr")
		tail   = job.Getenv("tail")
		follow = job.GetenvBool("follow")
		times  = job.GetenvBool("timestamps")
		lines  = -1
		format string
	)
	if !(stdout || stderr) {
		return fmt.Errorf("You must choose at least one stream")
	}
	if times {
		format = timeutils.RFC3339NanoFixed
	}
	if tail == "" {
		tail = "all"
	}
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}
	if container.LogDriverType() != "json-file" {
		return fmt.Errorf("\"logs\" endpoint is supported only for \"json-file\" logging driver")
	}
	cLog, err := container.ReadLog("json")
	if err != nil && os.IsNotExist(err) {
		// Legacy logs
		logrus.Debugf("Old logs format")
		if stdout {
			cLog, err := container.ReadLog("stdout")
			if err != nil {
				logrus.Errorf("Error reading logs (stdout): %s", err)
			} else if _, err := io.Copy(job.Stdout, cLog); err != nil {
				logrus.Errorf("Error streaming logs (stdout): %s", err)
			}
		}
		if stderr {
			cLog, err := container.ReadLog("stderr")
			if err != nil {
				logrus.Errorf("Error reading logs (stderr): %s", err)
			} else if _, err := io.Copy(job.Stderr, cLog); err != nil {
				logrus.Errorf("Error streaming logs (stderr): %s", err)
			}
		}
	} else if err != nil {
		logrus.Errorf("Error reading logs (json): %s", err)
	} else {
		if tail != "all" {
			var err error
			lines, err = strconv.Atoi(tail)
			if err != nil {
				logrus.Errorf("Failed to parse tail %s, error: %v, show all logs", tail, err)
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
				if times {
					// format can be "" or time format, so here can't be error
					logLine, _ = l.Format(format)
				}
				if l.Stream == "stdout" && stdout {
					io.WriteString(job.Stdout, logLine)
				}
				if l.Stream == "stderr" && stderr {
					io.WriteString(job.Stderr, logLine)
				}
				l.Reset()
			}
		}
	}
	if follow && container.IsRunning() {
		errors := make(chan error, 2)
		wg := sync.WaitGroup{}

		if stdout {
			wg.Add(1)
			stdoutPipe := container.StdoutLogPipe()
			defer stdoutPipe.Close()
			go func() {
				errors <- jsonlog.WriteLog(stdoutPipe, job.Stdout, format)
				wg.Done()
			}()
		}
		if stderr {
			wg.Add(1)
			stderrPipe := container.StderrLogPipe()
			defer stderrPipe.Close()
			go func() {
				errors <- jsonlog.WriteLog(stderrPipe, job.Stderr, format)
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
