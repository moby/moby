package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/tailfile"
	"github.com/docker/docker/pkg/timeutils"
)

func (daemon *Daemon) ContainerLogs(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}

	var (
		name      = job.Args[0]
		stdout    = job.GetenvBool("stdout")
		stderr    = job.GetenvBool("stderr")
		tail      = job.Getenv("tail")
		since     = job.Getenv("since")
		follow    = job.GetenvBool("follow")
		times     = job.GetenvBool("timestamps")
		lines     = -1
		format    string
		sinceDate time.Time
	)
	if !(stdout || stderr) {
		return job.Errorf("You must choose at least one stream")
	}
	if times {
		format = timeutils.RFC3339NanoFixed
	}
	if tail == "" {
		tail = "all"
	}
	if since != "" {
		var err error
		sinceDate, err = parseSinceDate(since)
		if err != nil {
			return job.Errorf("Error parsing since: %s", err)
		}
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
			l := &jsonlog.JSONLog{}
			for {
				if err := dec.Decode(l); err == io.EOF {
					break
				} else if err != nil {
					log.Errorf("Error streaming logs: %s", err)
					break
				}
				if since != "" && l.Created.Before(sinceDate) {
					continue
				}
				logLine := l.Log
				if times {
					logLine = fmt.Sprintf("%s %s", l.Created.Format(format), logLine)
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
				log.Errorf("%s", err)
			}
		}

	}
	return engine.StatusOK
}

func parseSinceDate(since string) (time.Time, error) {
	var format = "2006-01-02 15:04:05"
	parts := strings.FieldsFunc(since, func(c rune) bool {
		return c == '-' || c == ' ' || c == ':'
	})

	switch len(parts) {
	case 5: // date & time
		since = fmt.Sprintf("%s:00", since)
	case 3: // date
		since = fmt.Sprintf("%s 00:00:00", since)
	case 2: // time
		y, m, d := time.Now().Date()
		since = fmt.Sprintf("%d-%02d-%02d %s:00", y, m, d, since)
	default:
		return time.Time{}, errors.New("invalid since format")
	}

	return time.ParseInLocation(format, since, time.Local)
}
