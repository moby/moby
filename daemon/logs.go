package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/pkg/tailfile"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/jsonlog"
)

func (daemon *Daemon) ContainerLogs(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
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
		return job.Errorf("You must choose at least one stream")
	}
	if times {
		format = time.RFC3339Nano
	}
	if tail == "" {
		tail = "all"
	}
	container := daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}
	cLog, err := container.ReadLog("json")
	if err != nil && os.IsNotExist(err) {
		// Legacy logs
		log.Debugf("Old logs format")
		if stdout {
			cLog, err := container.ReadLog("stdout")
			if err != nil {
				log.Errorf("Error reading logs (stdout): %s", err)
			} else if _, err := io.Copy(job.Stdout, cLog); err != nil {
				log.Errorf("Error streaming logs (stdout): %s", err)
			}
		}
		if stderr {
			cLog, err := container.ReadLog("stderr")
			if err != nil {
				log.Errorf("Error reading logs (stderr): %s", err)
			} else if _, err := io.Copy(job.Stderr, cLog); err != nil {
				log.Errorf("Error streaming logs (stderr): %s", err)
			}
		}
	} else if err != nil {
		log.Errorf("Error reading logs (json): %s", err)
	} else {
		if tail != "all" {
			var err error
			lines, err = strconv.Atoi(tail)
			if err != nil {
				log.Errorf("Failed to parse tail %s, error: %v, show all logs", tail, err)
				lines = -1
			}
		}
		if lines != 0 {
			if lines > 0 {
				f := cLog.(*os.File)
				ls, err := tailfile.TailFile(f, lines)
				if err != nil {
					return job.Error(err)
				}
				tmp := bytes.NewBuffer([]byte{})
				for _, l := range ls {
					fmt.Fprintf(tmp, "%s\n", l)
				}
				cLog = tmp
			}
			dec := json.NewDecoder(cLog)
			for {
				l := &jsonlog.JSONLog{}

				if err := dec.Decode(l); err == io.EOF {
					break
				} else if err != nil {
					log.Errorf("Error streaming logs: %s", err)
					break
				}
				logLine := l.Log
				if times {
					logLine = fmt.Sprintf("%s %s", l.Created.Format(format), logLine)
				}
				if l.Stream == "stdout" && stdout {
					fmt.Fprintf(job.Stdout, "%s", logLine)
				}
				if l.Stream == "stderr" && stderr {
					fmt.Fprintf(job.Stderr, "%s", logLine)
				}
			}
		}
	}
	if follow && container.State.IsRunning() {
		errors := make(chan error, 2)
		if stdout {
			stdoutPipe := container.StdoutLogPipe()
			go func() {
				errors <- jsonlog.WriteLog(stdoutPipe, job.Stdout, format)
			}()
		}
		if stderr {
			stderrPipe := container.StderrLogPipe()
			go func() {
				errors <- jsonlog.WriteLog(stderrPipe, job.Stderr, format)
			}()
		}
		err := <-errors
		if err != nil {
			log.Errorf("%s", err)
		}
	}
	return engine.StatusOK
}
