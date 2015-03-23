package engine

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/common"
	"github.com/docker/docker/pkg/ioutils"
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
	handlers     map[string]Handler
	catchall     Handler
	hack         Hack // data for temporary hackery (see hack.go)
	id           string
	Stdout       io.Writer
	Stderr       io.Writer
	Stdin        io.Reader
	Logging      bool
	tasks        sync.WaitGroup
	l            sync.RWMutex // lock for shutdown
	shutdownWait sync.WaitGroup
	shutdown     bool
	onShutdown   []func() // shutdown handlers
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
		id:       common.RandomString(),
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
		Eng:     eng,
		Name:    name,
		Args:    args,
		Stdin:   NewInput(),
		Stdout:  NewOutput(),
		Stderr:  NewOutput(),
		env:     &Env{},
		closeIO: true,
	}
	if eng.Logging {
		job.Stderr.Add(ioutils.NopWriteCloser(eng.Stderr))
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

// OnShutdown registers a new callback to be called by Shutdown.
// This is typically used by services to perform cleanup.
func (eng *Engine) OnShutdown(h func()) {
	eng.l.Lock()
	eng.onShutdown = append(eng.onShutdown, h)
	eng.shutdownWait.Add(1)
	eng.l.Unlock()
}

// Shutdown permanently shuts down eng as follows:
// - It refuses all new jobs, permanently.
// - It waits for all active jobs to complete (with no timeout)
// - It calls all shutdown handlers concurrently (if any)
// - It returns when all handlers complete, or after 15 seconds,
//	whichever happens first.
func (eng *Engine) Shutdown() {
	eng.l.Lock()
	if eng.shutdown {
		eng.l.Unlock()
		eng.shutdownWait.Wait()
		return
	}
	eng.shutdown = true
	eng.l.Unlock()
	// We don't need to protect the rest with a lock, to allow
	// for other calls to immediately fail with "shutdown" instead
	// of hanging for 15 seconds.
	// This requires all concurrent calls to check for shutdown, otherwise
	// it might cause a race.

	// Wait for all jobs to complete.
	// Timeout after 5 seconds.
	tasksDone := make(chan struct{})
	go func() {
		eng.tasks.Wait()
		close(tasksDone)
	}()
	select {
	case <-time.After(time.Second * 5):
	case <-tasksDone:
	}

	// Call shutdown handlers, if any.
	// Timeout after 10 seconds.
	for _, h := range eng.onShutdown {
		go func(h func()) {
			h()
			eng.shutdownWait.Done()
		}(h)
	}
	done := make(chan struct{})
	go func() {
		eng.shutdownWait.Wait()
		close(done)
	}()
	select {
	case <-time.After(time.Second * 10):
	case <-done:
	}
	return
}

// IsShutdown returns true if the engine is in the process
// of shutting down, or already shut down.
// Otherwise it returns false.
func (eng *Engine) IsShutdown() bool {
	eng.l.RLock()
	defer eng.l.RUnlock()
	return eng.shutdown
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
