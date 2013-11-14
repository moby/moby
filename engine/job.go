package engine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
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
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	handler func(*Job) string
	status  string
	onExit  []func()
}

// Run executes the job and blocks until the job completes.
// If the job returns a failure status, an error is returned
// which includes the status.
func (job *Job) Run() error {
	defer func() {
		var wg sync.WaitGroup
		for _, f := range job.onExit {
			wg.Add(1)
			go func(f func()) {
				f()
				wg.Done()
			}(f)
		}
		wg.Wait()
	}()
	if job.Stdout != nil && job.Stdout != os.Stdout {
		job.Stdout = io.MultiWriter(job.Stdout, os.Stdout)
	}
	if job.Stderr != nil && job.Stderr != os.Stderr {
		job.Stderr = io.MultiWriter(job.Stderr, os.Stderr)
	}
	job.Eng.Logf("+job %s", job.CallString())
	defer func() {
		job.Eng.Logf("-job %s%s", job.CallString(), job.StatusString())
	}()
	if job.handler == nil {
		job.status = "command not found"
	} else {
		job.status = job.handler(job)
	}
	if job.status != "0" {
		return fmt.Errorf("%s: %s", job.Name, job.status)
	}
	return nil
}

func (job *Job) StdoutParseLines(dst *[]string, limit int) {
	job.parseLines(job.StdoutPipe(), dst, limit)
}

func (job *Job) StderrParseLines(dst *[]string, limit int) {
	job.parseLines(job.StderrPipe(), dst, limit)
}

func (job *Job) parseLines(src io.Reader, dst *[]string, limit int) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(src)
		for scanner.Scan() {
			// If the limit is reached, flush the rest of the source and return
			if limit > 0 && len(*dst) >= limit {
				io.Copy(ioutil.Discard, src)
				return
			}
			line := scanner.Text()
			// Append the line (with delimitor removed)
			*dst = append(*dst, line)
		}
	}()
	job.onExit = append(job.onExit, wg.Wait)
}

func (job *Job) StdoutParseString(dst *string) {
	lines := make([]string, 0, 1)
	job.StdoutParseLines(&lines, 1)
	job.onExit = append(job.onExit, func() {
		if len(lines) >= 1 {
			*dst = lines[0]
		}
	})
}

func (job *Job) StderrParseString(dst *string) {
	lines := make([]string, 0, 1)
	job.StderrParseLines(&lines, 1)
	job.onExit = append(job.onExit, func() { *dst = lines[0] })
}

func (job *Job) StdoutPipe() io.ReadCloser {
	r, w := io.Pipe()
	job.Stdout = w
	job.onExit = append(job.onExit, func() { w.Close() })
	return r
}

func (job *Job) StderrPipe() io.ReadCloser {
	r, w := io.Pipe()
	job.Stderr = w
	job.onExit = append(job.onExit, func() { w.Close() })
	return r
}

func (job *Job) CallString() string {
	return fmt.Sprintf("%s(%s)", job.Name, strings.Join(job.Args, ", "))
}

func (job *Job) StatusString() string {
	// FIXME: if a job returns the empty string, it will be printed
	// as not having returned.
	// (this only affects String which is a convenience function).
	if job.status != "" {
		var okerr string
		if job.status == "0" {
			okerr = "OK"
		} else {
			okerr = "ERR"
		}
		return fmt.Sprintf(" = %s (%s)", okerr, job.status)
	}
	return ""
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
	job.env = append(job.env, key+"="+value)
}

// DecodeEnv decodes `src` as a json dictionary, and adds
// each decoded key-value pair to the environment.
//
// If `text` cannot be decoded as a json dictionary, an error
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
