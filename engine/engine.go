package engine

import (
	"bufio"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"
)

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
	root     string
	handlers map[string]Handler
	hack     Hack // data for temporary hackery (see hack.go)
	id       string
	Stdout   io.Writer
	Stderr   io.Writer
	Stdin    io.Reader
	Logging  bool
}

func (eng *Engine) Root() string {
	return eng.root
}

func (eng *Engine) Register(name string, handler Handler) error {
	_, exists := eng.handlers[name]
	if exists {
		return fmt.Errorf("Can't overwrite handler for command %s", name)
	}
	eng.handlers[name] = handler
	return nil
}

// New initializes a new engine managing the directory specified at `root`.
// `root` is used to store containers and any other state private to the engine.
// Changing the contents of the root without executing a job will cause unspecified
// behavior.
func New(root string) (*Engine, error) {
	// Check for unsupported architectures
	if runtime.GOARCH != "amd64" {
		return nil, fmt.Errorf("The docker runtime currently only supports amd64 (not %s). This will change in the future. Aborting.", runtime.GOARCH)
	}
	// Check for unsupported kernel versions
	// FIXME: it would be cleaner to not test for specific versions, but rather
	// test for specific functionalities.
	// Unfortunately we can't test for the feature "does not cause a kernel panic"
	// without actually causing a kernel panic, so we need this workaround until
	// the circumstances of pre-3.8 crashes are clearer.
	// For details see http://github.com/dotcloud/docker/issues/407
	if k, err := utils.GetKernelVersion(); err != nil {
		log.Printf("WARNING: %s\n", err)
	} else {
		if utils.CompareKernelVersion(k, &utils.KernelVersionInfo{Kernel: 3, Major: 8, Minor: 0}) < 0 {
			if os.Getenv("DOCKER_NOWARN_KERNEL_VERSION") == "" {
				log.Printf("WARNING: You are running linux kernel version %s, which might be unstable running docker. Please upgrade your kernel to 3.8.0.", k.String())
			}
		}
	}

	if err := os.MkdirAll(root, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	eng := &Engine{
		root:     root,
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
	return eng, nil
}

func (eng *Engine) String() string {
	return fmt.Sprintf("%s|%s", eng.Root(), eng.id[:8])
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
	handler, exists := eng.handlers[name]
	if exists {
		job.handler = handler
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
