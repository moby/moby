package exec

import (
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
)

// Config holds the configurations for execs. The Daemon keeps
// track of both running and finished execs so that they can be
// examined both during and after completion.
type Config struct {
	sync.Mutex
	*runconfig.StreamConfig
	ID            string
	Running       bool
	ExitCode      *int
	ProcessConfig *execdriver.ProcessConfig
	OpenStdin     bool
	OpenStderr    bool
	OpenStdout    bool
	CanRemove     bool
	ContainerID   string
	DetachKeys    []byte

	// waitStart will be closed immediately after the exec is really started.
	waitStart chan struct{}

	// waitResize will be closed after Resize is finished.
	waitResize chan struct{}
}

// NewConfig initializes the a new exec configuration
func NewConfig() *Config {
	return &Config{
		ID:           stringid.GenerateNonCryptoID(),
		StreamConfig: runconfig.NewStreamConfig(),
		waitStart:    make(chan struct{}),
		waitResize:   make(chan struct{}),
	}
}

// Store keeps track of the exec configurations.
type Store struct {
	commands map[string]*Config
	sync.RWMutex
}

// NewStore initializes a new exec store.
func NewStore() *Store {
	return &Store{commands: make(map[string]*Config, 0)}
}

// Commands returns the exec configurations in the store.
func (e *Store) Commands() map[string]*Config {
	e.RLock()
	commands := make(map[string]*Config, len(e.commands))
	for id, config := range e.commands {
		commands[id] = config
	}
	e.RUnlock()
	return commands
}

// Add adds a new exec configuration to the store.
func (e *Store) Add(id string, Config *Config) {
	e.Lock()
	e.commands[id] = Config
	e.Unlock()
}

// Get returns an exec configuration by its id.
func (e *Store) Get(id string) *Config {
	e.RLock()
	res := e.commands[id]
	e.RUnlock()
	return res
}

// Delete removes an exec configuration from the store.
func (e *Store) Delete(id string) {
	e.Lock()
	delete(e.commands, id)
	e.Unlock()
}

// List returns the list of exec ids in the store.
func (e *Store) List() []string {
	var IDs []string
	e.RLock()
	for id := range e.commands {
		IDs = append(IDs, id)
	}
	e.RUnlock()
	return IDs
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

// WaitResize waits until terminal resize finishes or time out.
func (c *Config) WaitResize() error {
	select {
	case <-c.waitResize:
	case <-time.After(time.Second):
		return fmt.Errorf("Terminal resize for exec %s time out.", c.ID)
	}
	return nil
}

// Close closes the wait channel for the progress.
func (c *Config) Close() {
	close(c.waitStart)
}

// CloseResize closes the wait channel for resizing terminal.
func (c *Config) CloseResize() {
	close(c.waitResize)
}

// Resize changes the size of the terminal for the exec process.
func (c *Config) Resize(h, w int) error {
	defer c.CloseResize()
	select {
	case <-c.waitStart:
	case <-time.After(time.Second):
		return fmt.Errorf("Exec %s is not running, so it can not be resized.", c.ID)
	}
	return c.ProcessConfig.Terminal.Resize(h, w)
}
