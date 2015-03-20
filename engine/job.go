package engine

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
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
// For status, 0 indicates success, and any other integers indicates an error.
// This allows for richer error reporting.
//
type Job struct {
	Eng     *Engine
	Name    string
	Args    []string
	env     *Env
	Stdout  *Output
	Stderr  *Output
	Stdin   *Input
	handler Handler
	status  Status
	end     time.Time
	closeIO bool
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
	if job.Eng.IsShutdown() && !job.GetenvBool("overrideShutdown") {
		return fmt.Errorf("engine is shutdown")
	}
	// FIXME: this is a temporary workaround to avoid Engine.Shutdown
	// waiting 5 seconds for server/api.ServeApi to complete (which it never will)
	// everytime the daemon is cleanly restarted.
	// The permanent fix is to implement Job.Stop and Job.OnStop so that
	// ServeApi can cooperate and terminate cleanly.
	if job.Name != "serveapi" {
		job.Eng.l.Lock()
		job.Eng.tasks.Add(1)
		job.Eng.l.Unlock()
		defer job.Eng.tasks.Done()
	}
	// FIXME: make this thread-safe
	// FIXME: implement wait
	if !job.end.IsZero() {
		return fmt.Errorf("%s: job has already completed", job.Name)
	}
	// Log beginning and end of the job
	if job.Eng.Logging {
		log.Infof("+job %s", job.CallString())
		defer func() {
			log.Infof("-job %s%s", job.CallString(), job.StatusString())
		}()
	}
	var errorMessage = bytes.NewBuffer(nil)
	job.Stderr.Add(errorMessage)
	if job.handler == nil {
		job.Errorf("%s: command not found", job.Name)
		job.status = 127
	} else {
		job.status = job.handler(job)
		job.end = time.Now()
	}
	if job.closeIO {
		// Wait for all background tasks to complete
		if err := job.Stdout.Close(); err != nil {
			return err
		}
		if err := job.Stderr.Close(); err != nil {
			return err
		}
		if err := job.Stdin.Close(); err != nil {
			return err
		}
	}
	if job.status != 0 {
		return fmt.Errorf("%s", Tail(errorMessage, 1))
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

func (job *Job) Env() *Env {
	return job.env
}

func (job *Job) EnvExists(key string) (value bool) {
	return job.env.Exists(key)
}

func (job *Job) Getenv(key string) (value string) {
	return job.env.Get(key)
}

func (job *Job) GetenvBool(key string) (value bool) {
	return job.env.GetBool(key)
}

func (job *Job) SetenvBool(key string, value bool) {
	job.env.SetBool(key, value)
}

func (job *Job) GetenvTime(key string) (value time.Time, err error) {
	return job.env.GetTime(key)
}

func (job *Job) SetenvTime(key string, value time.Time) {
	job.env.SetTime(key, value)
}

func (job *Job) GetenvSubEnv(key string) *Env {
	return job.env.GetSubEnv(key)
}

func (job *Job) SetenvSubEnv(key string, value *Env) error {
	return job.env.SetSubEnv(key, value)
}

func (job *Job) GetenvInt64(key string) int64 {
	return job.env.GetInt64(key)
}

func (job *Job) GetenvInt(key string) int {
	return job.env.GetInt(key)
}

func (job *Job) SetenvInt64(key string, value int64) {
	job.env.SetInt64(key, value)
}

func (job *Job) SetenvInt(key string, value int) {
	job.env.SetInt(key, value)
}

// Returns nil if key not found
func (job *Job) GetenvList(key string) []string {
	return job.env.GetList(key)
}

func (job *Job) GetenvJson(key string, iface interface{}) error {
	return job.env.GetJson(key, iface)
}

func (job *Job) SetenvJson(key string, value interface{}) error {
	return job.env.SetJson(key, value)
}

func (job *Job) SetenvList(key string, value []string) error {
	return job.env.SetJson(key, value)
}

func (job *Job) Setenv(key, value string) {
	job.env.Set(key, value)
}

// DecodeEnv decodes `src` as a json dictionary, and adds
// each decoded key-value pair to the environment.
//
// If `src` cannot be decoded as a json dictionary, an error
// is returned.
func (job *Job) DecodeEnv(src io.Reader) error {
	return job.env.Decode(src)
}

func (job *Job) EncodeEnv(dst io.Writer) error {
	return job.env.Encode(dst)
}

func (job *Job) ImportEnv(src interface{}) (err error) {
	return job.env.Import(src)
}

func (job *Job) Environ() map[string]string {
	return job.env.Map()
}

func (job *Job) Logf(format string, args ...interface{}) (n int, err error) {
	prefixedFormat := fmt.Sprintf("[%s] %s\n", job, strings.TrimRight(format, "\n"))
	return fmt.Fprintf(job.Stderr, prefixedFormat, args...)
}

func (job *Job) Printf(format string, args ...interface{}) (n int, err error) {
	return fmt.Fprintf(job.Stdout, format, args...)
}

func (job *Job) Errorf(format string, args ...interface{}) Status {
	if format[len(format)-1] != '\n' {
		format = format + "\n"
	}
	fmt.Fprintf(job.Stderr, format, args...)
	return StatusErr
}

func (job *Job) Error(err error) Status {
	fmt.Fprintf(job.Stderr, "%s\n", err)
	return StatusErr
}

func (job *Job) StatusCode() int {
	return int(job.status)
}

func (job *Job) SetCloseIO(val bool) {
	job.closeIO = val
}
