package engine

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/dotcloud/docker/utils"
)

// Installer is a standard interface for objects which can "install" themselves
// on an engine by registering handlers.
// This can be used as an entrypoint for external plugins etc.
type Installer interface {
	Install(*Engine) error
}

type Handler func(*Job) Status

var globalHandlers map[string]Handler

func init() {
	globalHandlers = make(map[string]Handler)
}

func Register(name string, handler Handler) error {
	_, exists := globalHandlers[name]
	if exists {
		return fmt.Errorf("Can't overwrite global handler for command %s", name)
	}
	globalHandlers[name] = handler
	return nil
}

func unregister(name string) {
	delete(globalHandlers, name)
}

// The Engine is the core of Docker.
// It acts as a store for *containers*, and allows manipulation of these
// containers by executing *jobs*.
type Engine struct {
	handlers map[string]Handler
	catchall Handler
	hack     Hack // data for temporary hackery (see hack.go)
	id       string
	Stdout   io.Writer
	Stderr   io.Writer
	Stdin    io.Reader
	Logging  bool
}

func (eng *Engine) Register(name string, handler Handler) error {
	_, exists := eng.handlers[name]
	if exists {
		return fmt.Errorf("Can't overwrite handler for command %s", name)
	}
	eng.handlers[name] = handler
	return nil
}

func (eng *Engine) RegisterCatchall(catchall Handler) {
	eng.catchall = catchall
}

// New initializes a new engine.
func New() *Engine {
	eng := &Engine{
		handlers: make(map[string]Handler),
		id:       utils.RandomString(),
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
		Stdin:    os.Stdin,
		Logging:  true,
	}
	eng.Register("commands", func(job *Job) Status {
		for _, name := range eng.commands() {
			job.Printf("%s\n", name)
		}
		return StatusOK
	})
	// Copy existing global handlers
	for k, v := range globalHandlers {
		eng.handlers[k] = v
	}
	return eng
}

func (eng *Engine) String() string {
	return fmt.Sprintf("%s", eng.id[:8])
}

// Commands returns a list of all currently registered commands,
// sorted alphabetically.
func (eng *Engine) commands() []string {
	names := make([]string, 0, len(eng.handlers))
	for name := range eng.handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Job creates a new job which can later be executed.
// This function mimics `Command` from the standard os/exec package.
func (eng *Engine) Job(name string, args ...string) *Job {
	job := &Job{
		Eng:    eng,
		Name:   name,
		Args:   args,
		Stdin:  NewInput(),
		Stdout: NewOutput(),
		Stderr: NewOutput(),
		env:    &Env{},
	}
	if eng.Logging {
		job.Stderr.Add(utils.NopWriteCloser(eng.Stderr))
	}

	// Catchall is shadowed by specific Register.
	if handler, exists := eng.handlers[name]; exists {
		job.handler = handler
	} else if eng.catchall != nil && name != "" {
		// empty job names are illegal, catchall or not.
		job.handler = eng.catchall
	}
	return job
}

// ParseJob creates a new job from a text description using a shell-like syntax.
//
// The following syntax is used to parse `input`:
//
// * Words are separated using standard whitespaces as separators.
// * Quotes and backslashes are not interpreted.
// * Words of the form 'KEY=[VALUE]' are added to the job environment.
// * All other words are added to the job arguments.
//
// For example:
//
// job, _ := eng.ParseJob("VERBOSE=1 echo hello TEST=true world")
//
// The resulting job will have:
//	job.Args={"echo", "hello", "world"}
//	job.Env={"VERBOSE":"1", "TEST":"true"}
//
func (eng *Engine) ParseJob(input string) (*Job, error) {
	// FIXME: use a full-featured command parser
	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Split(bufio.ScanWords)
	var (
		cmd []string
		env Env
	)
	for scanner.Scan() {
		word := scanner.Text()
		kv := strings.SplitN(word, "=", 2)
		if len(kv) == 2 {
			env.Set(kv[0], kv[1])
		} else {
			cmd = append(cmd, word)
		}
	}
	if len(cmd) == 0 {
		return nil, fmt.Errorf("empty command: '%s'", input)
	}
	job := eng.Job(cmd[0], cmd[1:]...)
	job.Env().Init(&env)
	return job, nil
}

func (eng *Engine) Logf(format string, args ...interface{}) (n int, err error) {
	if !eng.Logging {
		return 0, nil
	}
	prefixedFormat := fmt.Sprintf("[%s] %s\n", eng, strings.TrimRight(format, "\n"))
	return fmt.Fprintf(eng.Stderr, prefixedFormat, args...)
}
