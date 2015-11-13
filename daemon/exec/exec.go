package exec

import (
	"sync"
	"time"

	"github.com/docker/docker/container/streams"
	"github.com/docker/docker/daemon/execdriver"
	derr "github.com/docker/docker/errors"
)

// Config holds the configurations for execs. The Daemon keeps
// track of both running and finished execs so that they can be
// examined both during and after completion.
type Config struct {
	sync.Mutex
	ID            string
	Running       bool
	ExitCode      int
	ProcessConfig *execdriver.ProcessConfig
	streams.StreamConfig
	OpenStdin   bool
	OpenStderr  bool
	OpenStdout  bool
	CanRemove   bool
	ContainerID string

	// waitStart will be closed immediately after the exec is really started.
	waitStart chan struct{}
}

// Store keeps track of the exec configurations.
type Store struct {
	s map[string]*Config
	sync.RWMutex
}

// NewStore initializes a new exec store.
func NewStore() *Store {
	return &Store{s: make(map[string]*Config, 0)}
}

// Configs returns the exec configurations in the store.
func (e *Store) Configs() map[string]*Config {
	return e.s
}

// Add adds a new exec configuration to the store.
func (e *Store) Add(id string, Config *Config) {
	e.Lock()
	e.s[id] = Config
	e.Unlock()
}

// Get returns an exec configuration by its id.
func (e *Store) Get(id string) *Config {
	e.RLock()
	res := e.s[id]
	e.RUnlock()
	return res
}

// Delete removes an exec configuration from the store.
func (e *Store) Delete(id string) {
	e.Lock()
	delete(e.s, id)
	e.Unlock()
}

// List returns the list of exec ids in the store.
func (e *Store) List() []string {
	var IDs []string
	e.RLock()
	for id := range e.s {
		IDs = append(IDs, id)
	}
	e.RUnlock()
	return IDs
}

// Init initializes the wait exec channel.
func (c *Config) Init() {
	c.waitStart = make(chan struct{})
}

// Wait waits until the exec process finishes or there is an error in the error channel.
func (c *Config) Wait(cErr chan error) error {
	// Exec should not return until the process is actually running
	select {
	case <-c.waitStart:
	case err := <-cErr:
		return err
	}
	return nil
}

// Close closes the wait channel for the progress.
func (c *Config) Close() {
	close(c.waitStart)
}

// Resize changes the size of the terminal for the exec process.
func (c *Config) Resize(h, w int) error {
	select {
	case <-c.waitStart:
	case <-time.After(time.Second):
		return derr.ErrorCodeExecResize.WithArgs(c.ID)
	}
	return c.ProcessConfig.Terminal.Resize(h, w)
}
