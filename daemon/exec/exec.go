package exec

import (
	"runtime"
	"sync"

	"github.com/containerd/containerd"
	"github.com/docker/docker/container/stream"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/pkg/stringid"
	"github.com/sirupsen/logrus"
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
	Env          []string
	Pid          int
}

// NewConfig initializes the a new exec configuration
func NewConfig() *Config {
	return &Config{
		ID:           stringid.GenerateNonCryptoID(),
		StreamConfig: stream.NewConfig(),
	}
}

type cio struct {
	containerd.IO

	sc *stream.Config
}

func (i *cio) Close() error {
	i.IO.Close()

	return i.sc.CloseStreams()
}

func (i *cio) Wait() {
	i.sc.Wait()

	i.IO.Wait()
}

// InitializeStdio is called by libcontainerd to connect the stdio.
func (c *Config) InitializeStdio(iop *libcontainerd.IOPipe) (containerd.IO, error) {
	c.StreamConfig.CopyToPipe(iop)

	if c.StreamConfig.Stdin() == nil && !c.Tty && runtime.GOOS == "windows" {
		if iop.Stdin != nil {
			if err := iop.Stdin.Close(); err != nil {
				logrus.Errorf("error closing exec stdin: %+v", err)
			}
		}
	}

	return &cio{IO: iop, sc: c.StreamConfig}, nil
}

// CloseStreams closes the stdio streams for the exec
func (c *Config) CloseStreams() error {
	return c.StreamConfig.CloseStreams()
}

// SetExitCode sets the exec config's exit code
func (c *Config) SetExitCode(code int) {
	c.ExitCode = &code
}

// Store keeps track of the exec configurations.
type Store struct {
	byID  map[string]*Config
	byPid map[int]*Config
	sync.RWMutex
}

// NewStore initializes a new exec store.
func NewStore() *Store {
	return &Store{
		byID:  make(map[string]*Config),
		byPid: make(map[int]*Config),
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

// SetPidUnlocked adds an association between a Pid and a config, it does not
// synchronized with other operations.
func (e *Store) SetPidUnlocked(id string, pid int) {
	if config, ok := e.byID[id]; ok {
		e.byPid[pid] = config
	}
}

// Get returns an exec configuration by its id.
func (e *Store) Get(id string) *Config {
	e.RLock()
	res := e.byID[id]
	e.RUnlock()
	return res
}

// ByPid returns an exec configuration by its pid.
func (e *Store) ByPid(pid int) *Config {
	e.RLock()
	res := e.byPid[pid]
	e.RUnlock()
	return res
}

// Delete removes an exec configuration from the store.
func (e *Store) Delete(id string, pid int) {
	e.Lock()
	delete(e.byPid, pid)
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
