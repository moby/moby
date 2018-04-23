package exec // import "github.com/docker/docker/daemon/exec"

import (
	"context"
	"runtime"
	"sync"

	"github.com/containerd/containerd/cio"
	"github.com/docker/docker/container/stream"
	"github.com/docker/docker/pkg/stringid"
	"github.com/sirupsen/logrus"
)

// WaitCondition is an enum type for different exec states to wait for.
type WaitCondition int

// Possible WaitCondition Values.
//
// WaitConditionRunning is used to wait for the exec to be running.
//
// WaitConditionExited is used to wait for the exec to exit.
const (
	WaitConditionRunning WaitCondition = iota
	WaitConditionExited
)

// Config holds the configurations for execs. The Daemon keeps
// track of both running and finished execs so that they can be
// examined both during and after completion.
type Config struct {
	sync.Mutex
	StreamConfig *stream.Config
	ID           string
	Running      bool
	ExitCode     *int
	OpenStdin    bool
	OpenStderr   bool
	OpenStdout   bool
	CanRemove    bool
	ContainerID  string
	DetachKeys   []byte
	Entrypoint   string
	Args         []string
	Tty          bool
	Privileged   bool
	User         string
	WorkingDir   string
	Env          []string
	Pid          int

	waitRunning chan struct{}
	waitExited  chan struct{}
}

// Status is used to return an exec wait results.
// Implements exec.ExitCode interface.
type Status struct {
	exitCode int
	err      error
}

// ExitCode returns current exitcode for the exec.
func (s Status) ExitCode() int {
	return s.exitCode
}

// Err returns current error for the state. Returns nil if the exec had exited
// on its own.
func (s Status) Err() error {
	return s.err
}

// NewConfig initializes the a new exec configuration
func NewConfig() *Config {
	return &Config{
		ID:           stringid.GenerateNonCryptoID(),
		StreamConfig: stream.NewConfig(),
		waitRunning:  make(chan struct{}),
		waitExited:   make(chan struct{}),
	}
}

type rio struct {
	cio.IO

	sc *stream.Config
}

func (i *rio) Close() error {
	i.IO.Close()

	return i.sc.CloseStreams()
}

func (i *rio) Wait() {
	i.sc.Wait()

	i.IO.Wait()
}

// InitializeStdio is called by libcontainerd to connect the stdio.
func (c *Config) InitializeStdio(iop *cio.DirectIO) (cio.IO, error) {
	c.StreamConfig.CopyToPipe(iop)

	if c.StreamConfig.Stdin() == nil && !c.Tty && runtime.GOOS == "windows" {
		if iop.Stdin != nil {
			if err := iop.Stdin.Close(); err != nil {
				logrus.Errorf("error closing exec stdin: %+v", err)
			}
		}
	}

	return &rio{IO: iop, sc: c.StreamConfig}, nil
}

// CloseStreams closes the stdio streams for the exec
func (c *Config) CloseStreams() error {
	return c.StreamConfig.CloseStreams()
}

// SetExitCode sets the exec config's exit code
func (c *Config) SetExitCode(code int) {
	c.ExitCode = &code
	c.Running = false
	close(c.waitExited)
}

// SetStarted trigger the WaitConditionRunning as used with Wait()
func (c *Config) SetStarted() {
	c.Lock()
	close(c.waitRunning)
	c.Unlock()
}

// Wait return a channel that can be used to check that the given condition
// came to pass
func (c *Config) Wait(ctx context.Context, cond WaitCondition) <-chan Status {
	statusCh := make(chan Status, 1)
	var waitCh chan struct{}

	if cond == WaitConditionRunning {
		waitCh = c.waitRunning
	} else {
		waitCh = c.waitExited
	}

	go func() {
		select {
		case <-ctx.Done():
			statusCh <- Status{
				exitCode: -1,
				err:      ctx.Err(),
			}
		case <-waitCh:
			c.Lock()
			ec := -1
			if c.ExitCode != nil {
				ec = *c.ExitCode
			}
			statusCh <- Status{
				exitCode: ec,
				err:      nil,
			}
			c.Unlock()
		}
	}()

	return statusCh
}

// Store keeps track of the exec configurations.
type Store struct {
	byID map[string]*Config
	sync.RWMutex
}

// NewStore initializes a new exec store.
func NewStore() *Store {
	return &Store{
		byID: make(map[string]*Config),
	}
}

// Commands returns the exec configurations in the store.
func (e *Store) Commands() map[string]*Config {
	e.RLock()
	byID := make(map[string]*Config, len(e.byID))
	for id, config := range e.byID {
		byID[id] = config
	}
	e.RUnlock()
	return byID
}

// Add adds a new exec configuration to the store.
func (e *Store) Add(id string, Config *Config) {
	e.Lock()
	e.byID[id] = Config
	e.Unlock()
}

// Get returns an exec configuration by its id.
func (e *Store) Get(id string) *Config {
	e.RLock()
	res := e.byID[id]
	e.RUnlock()
	return res
}

// Delete removes an exec configuration from the store.
func (e *Store) Delete(id string, pid int) {
	e.Lock()
	delete(e.byID, id)
	e.Unlock()
}

// List returns the list of exec ids in the store.
func (e *Store) List() []string {
	var IDs []string
	e.RLock()
	for id := range e.byID {
		IDs = append(IDs, id)
	}
	e.RUnlock()
	return IDs
}
