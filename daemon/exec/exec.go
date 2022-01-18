package exec // import "github.com/docker/docker/daemon/exec"

import (
	"context"
	"sync"

	"github.com/containerd/containerd/cio"
	"github.com/docker/docker/container/stream/streamv2"
	"github.com/docker/docker/pkg/stringid"
)

// Config holds the configurations for execs. The Daemon keeps
// track of both running and finished execs so that they can be
// examined both during and after completion.
type Config struct {
	sync.Mutex
	Started     chan struct{}
	ID          string
	Running     bool
	ExitCode    *int
	OpenStdin   bool
	OpenStderr  bool
	OpenStdout  bool
	CanRemove   bool
	ContainerID string
	DetachKeys  []byte
	Entrypoint  string
	Args        []string
	Tty         bool
	Privileged  bool
	User        string
	WorkingDir  string
	Env         []string
	Pid         int

	streams *streamv2.Streams
	wg      sync.WaitGroup
}

// NewConfig initializes the a new exec configuration
func NewConfig(streams *streamv2.Streams) *Config {
	return &Config{
		ID:      stringid.GenerateRandomID(),
		Started: make(chan struct{}),
	}
}

// InitializeStdio is called by libcontainerd to connect the stdio.
func (c *Config) InitializeStdio(iop *cio.DirectIO) (cio.IO, error) {
	if err := c.streams.OpenProcessStreams(context.TODO(), c.ID, iop); err != nil {
		return nil, err
	}
	c.wg.Add(1)
	return &stdioStream{IO: iop, ec: c}, nil
}

// CloseStreams closes the stdio streams for the exec
func (c *Config) CloseStreams() error {
	err := c.streams.CloseProcessStreams(context.TODO(), c.ID)
	if err != nil {
		c.wg.Done()
	}
	return err
}

func (c *Config) waitStreams() {
	c.wg.Wait()
}

// SetExitCode sets the exec config's exit code
func (c *Config) SetExitCode(code int) {
	c.ExitCode = &code
}

var _ cio.IO = &stdioStream{}

type stdioStream struct {
	cio.IO
	ec *Config
}

func (s *stdioStream) Close() error {
	s.IO.Close()
	return s.ec.CloseStreams()
}

func (s *stdioStream) Wait() {
	s.ec.waitStreams()
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
