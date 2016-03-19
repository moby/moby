package libcontainerd

import "sync"

type remote struct {
}

func (r *remote) Client(b Backend) (Client, error) {
	c := &client{
		clientCommon: clientCommon{
			backend:          b,
			containerMutexes: make(map[string]*sync.Mutex),
			containers:       make(map[string]*container),
		},
	}
	return c, nil
}

// Cleanup is a no-op on Windows. It is here to implement the interface.
func (r *remote) Cleanup() {
}

// New creates a fresh instance of libcontainerd remote. On Windows,
// this is not used as there is no remote containerd process.
func New(_ string, _ ...RemoteOption) (Remote, error) {
	return &remote{}, nil
}
