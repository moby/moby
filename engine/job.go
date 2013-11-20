package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
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
	Eng     *Engine
	Name    string
	Args    []string
	env     []string
	Stdout  *Output
	Stderr  *Output
	Stdin   *Input
	handler Handler
	status  Status
	end     time.Time
	onExit  []func()
}

type Status int

const (
	StatusOK       Status = 0
	StatusErr      Status = 1
	StatusNotFound Status = 127
)

// Run executes the job and blocks until the job completes.
// If the job returns a failure status, an error is returned
// which includes the status.
func (job *Job) Run() error {
	// FIXME: make this thread-safe
	// FIXME: implement wait
	if !job.end.IsZero() {
		return fmt.Errorf("%s: job has already completed", job.Name)
	}
	// Log beginning and end of the job
	job.Eng.Logf("+job %s", job.CallString())
	defer func() {
		job.Eng.Logf("-job %s%s", job.CallString(), job.StatusString())
	}()
	var errorMessage string
	job.Stderr.AddString(&errorMessage)
	if job.handler == nil {
		job.Errorf("%s: command not found", job.Name)
		job.status = 127
	} else {
		job.status = job.handler(job)
		job.end = time.Now()
	}
	// Wait for all background tasks to complete
	if err := job.Stdout.Close(); err != nil {
		return err
	}
	if err := job.Stderr.Close(); err != nil {
		return err
	}
	if job.status != 0 {
		return fmt.Errorf("%s: %s", job.Name, errorMessage)
	}
	return nil
}

func (job *Job) CallString() string {
	return fmt.Sprintf("%s(%s)", job.Name, strings.Join(job.Args, ", "))
}

func (job *Job) StatusString() string {
	// If the job hasn't completed, status string is empty
	if job.end.IsZero() {
		return ""
	}
	var okerr string
	if job.status == StatusOK {
		okerr = "OK"
	} else {
		okerr = "ERR"
	}
	return fmt.Sprintf(" = %s (%d)", okerr, job.status)
}

// String returns a human-readable description of `job`
func (job *Job) String() string {
	return fmt.Sprintf("%s.%s%s", job.Eng, job.CallString(), job.StatusString())
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

func (job *Job) GetenvInt(key string) int64 {
	s := strings.Trim(job.Getenv(key), " \t")
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return -1
	}
	return val
}

func (job *Job) SetenvInt(key string, value int64) {
	job.Setenv(key, fmt.Sprintf("%d", value))
}

// Returns nil if key not found
func (job *Job) GetenvList(key string) []string {
	sval := job.Getenv(key)
	if sval == "" {
		return nil
	}
	l := make([]string, 0, 1)
	if err := json.Unmarshal([]byte(sval), &l); err != nil {
		l = append(l, sval)
	}
	return l
}

func (job *Job) SetenvJson(key string, value interface{}) error {
	sval, err := json.Marshal(value)
	if err != nil {
		return err
	}
	job.Setenv(key, string(sval))
	return nil
}

func (job *Job) SetenvList(key string, value []string) error {
	return job.SetenvJson(key, value)
}

func (job *Job) Setenv(key, value string) {
	job.env = append(job.env, key+"="+value)
}

// DecodeEnv decodes `src` as a json dictionary, and adds
// each decoded key-value pair to the environment.
//
// If `src` cannot be decoded as a json dictionary, an error
// is returned.
func (job *Job) DecodeEnv(src io.Reader) error {
	m := make(map[string]interface{})
	if err := json.NewDecoder(src).Decode(&m); err != nil {
		return err
	}
	for k, v := range m {
		// FIXME: we fix-convert float values to int, because
		// encoding/json decodes integers to float64, but cannot encode them back.
		// (See http://golang.org/src/pkg/encoding/json/decode.go#L46)
		if fval, ok := v.(float64); ok {
			job.SetenvInt(k, int64(fval))
		} else if sval, ok := v.(string); ok {
			job.Setenv(k, sval)
		} else if val, err := json.Marshal(v); err == nil {
			job.Setenv(k, string(val))
		} else {
			job.Setenv(k, fmt.Sprintf("%v", v))
		}
	}
	return nil
}

func (job *Job) EncodeEnv(dst io.Writer) error {
	m := make(map[string]interface{})
	for k, v := range job.Environ() {
		var val interface{}
		if err := json.Unmarshal([]byte(v), &val); err == nil {
			// FIXME: we fix-convert float values to int, because
			// encoding/json decodes integers to float64, but cannot encode them back.
			// (See http://golang.org/src/pkg/encoding/json/decode.go#L46)
			if fval, isFloat := val.(float64); isFloat {
				val = int(fval)
			}
			m[k] = val
		} else {
			m[k] = v
		}
	}
	if err := json.NewEncoder(dst).Encode(&m); err != nil {
		return err
	}
	return nil
}

func (job *Job) ExportEnv(dst interface{}) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("ExportEnv %s", err)
		}
	}()
	var buf bytes.Buffer
	// step 1: encode/marshal the env to an intermediary json representation
	if err := job.EncodeEnv(&buf); err != nil {
		return err
	}
	// step 2: decode/unmarshal the intermediary json into the destination object
	if err := json.NewDecoder(&buf).Decode(dst); err != nil {
		return err
	}
	return nil
}

func (job *Job) ImportEnv(src interface{}) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("ImportEnv: %s", err)
		}
	}()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(src); err != nil {
		return err
	}
	if err := job.DecodeEnv(&buf); err != nil {
		return err
	}
	return nil
}

func (job *Job) Environ() map[string]string {
	m := make(map[string]string)
	for _, kv := range job.env {
		parts := strings.SplitN(kv, "=", 2)
		m[parts[0]] = parts[1]
	}
	return m
}

func (job *Job) Logf(format string, args ...interface{}) (n int, err error) {
	prefixedFormat := fmt.Sprintf("[%s] %s\n", job, strings.TrimRight(format, "\n"))
	return fmt.Fprintf(job.Stderr, prefixedFormat, args...)
}

func (job *Job) Printf(format string, args ...interface{}) (n int, err error) {
	return fmt.Fprintf(job.Stdout, format, args...)
}

func (job *Job) Errorf(format string, args ...interface{}) (n int, err error) {
	return fmt.Fprintf(job.Stderr, format, args...)
}

func (job *Job) Error(err error) (int, error) {
	return fmt.Fprintf(job.Stderr, "%s", err)
}
