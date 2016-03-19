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

// Cleanup is a no-op on Windows. It is here to implement the same interface
// to meet compilation requirements.
func (r *remote) Cleanup() {
}

// New creates a fresh instance of libcontainerd remote. This is largely
// a no-op on Windows.
func New(_ string, _ ...RemoteOption) (Remote, error) {
	return &remote{}, nil
}
