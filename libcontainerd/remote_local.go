// +build windows

package libcontainerd

import (
	"sync"

	"github.com/sirupsen/logrus"
)

type remote struct {
	sync.RWMutex

	logger  *logrus.Entry
	clients []*client

	// Options
	rootDir  string
	stateDir string
}

// New creates a fresh instance of libcontainerd remote.
func New(rootDir, stateDir string, options ...RemoteOption) (Remote, error) {
	return &remote{
		logger:   logrus.WithField("module", "libcontainerd"),
		rootDir:  rootDir,
		stateDir: stateDir,
	}, nil
}

type client struct {
	sync.Mutex

	rootDir    string
	stateDir   string
	backend    Backend
	logger     *logrus.Entry
	eventQ     queue
	containers map[string]*container
}

func (r *remote) NewClient(ns string, b Backend) (Client, error) {
	c := &client{
		rootDir:    r.rootDir,
		stateDir:   r.stateDir,
		backend:    b,
		logger:     r.logger.WithField("namespace", ns),
		containers: make(map[string]*container),
	}
	r.Lock()
	r.clients = append(r.clients, c)
	r.Unlock()

	return c, nil
}

func (r *remote) Cleanup() {
	// Nothing to do
}
