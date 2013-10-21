package engine

import (
	"io"
	"strings"
	"fmt"
	"encoding/json"
)

// A job is the fundamental unit of work in the docker engine.
// Everything docker can do should eventually be exposed as a job.
// For example: execute a process in a container, create a new container,
// download an archive from the internet, serve the http api, etc.
//
// The job API is designed after unix processes: a job has a name, arguments,
// environment variables, standard streams for input, output and error, and
// an exit status which can indicate success (0) or error (anything else).
//
// One slight variation is that jobs report their status as a string. The
// string "0" indicates success, and any other strings indicates an error.
// This allows for richer error reporting.
// 
type Job struct {
	eng	*Engine
	Name	string
	Args	[]string
	env	[]string
	Stdin	io.ReadCloser
	Stdout	io.WriteCloser
	Stderr	io.WriteCloser
	handler	func(*Job) string
	status	string
}

// Run executes the job and blocks until the job completes.
// If the job returns a failure status, an error is returned
// which includes the status.
func (job *Job) Run() error {
	if job.handler == nil {
		return fmt.Errorf("Undefined job handler")
	}
	status := job.handler(job)
	job.status = status
	if status != "0" {
		return fmt.Errorf("Job failed with status %s", status)
	}
	return nil
}


func (job *Job) Getenv(key string) (value string) {
        for _, kv := range job.env {
                if strings.Index(kv, "=") == -1 {
                        continue
                }
                parts := strings.SplitN(kv, "=", 2)
                if parts[0] != key {
                        continue
                }
                if len(parts) < 2 {
                        value = ""
                } else {
                        value = parts[1]
                }
        }
        return
}

func (job *Job) GetenvBool(key string) (value bool) {
	s := strings.ToLower(strings.Trim(job.Getenv(key), " \t"))
	if s == "" || s == "0" || s == "no" || s == "false" || s == "none" {
		return false
	}
	return true
}

func (job *Job) SetenvBool(key string, value bool) {
	if value {
		job.Setenv(key, "1")
	} else {
		job.Setenv(key, "0")
	}
}

func (job *Job) GetenvList(key string) []string {
	sval := job.Getenv(key)
	l := make([]string, 0, 1)
	if err := json.Unmarshal([]byte(sval), &l); err != nil {
		l = append(l, sval)
	}
	return l
}

func (job *Job) SetenvList(key string, value []string) error {
	sval, err := json.Marshal(value)
	if err != nil {
		return err
	}
	job.Setenv(key, string(sval))
	return nil
}

func (job *Job) Setenv(key, value string) {
	job.env = append(job.env, key + "=" + value)
}
