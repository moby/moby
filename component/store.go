package component

import (
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

var (
	existsErr       = errors.New("component exists")
	nilComponentErr = errors.New("cannot store nil component")

	defaultStore = NewStore()
)

// Component is the type stored in the component store
type Component interface{}

// NewStore creates a new
func NewStore() *Store {
	return &Store{
		components: make(map[string]Component),
		waiters:    make(map[string]map[chan Component]struct{}),
	}
}

// Store stores components.
type Store struct {
	mu         sync.Mutex
	components map[string]Component
	waiters    map[string]map[chan Component]struct{}
}

// Register registers a component with the default store
func Register(name string, c Component) (cancel func(), err error) {
	return defaultStore.Register(name, c)
}

// Get looks up the passed in component name in the default store.
// Get returns nil if the component does not exist.
func Get(name string) Component {
	return defaultStore.Get(name)
}

// Wait waits for a component with the given name in the default store
func Wait(ctx context.Context, name string) Component {
	return defaultStore.Wait(ctx, name)
}

// Register registers a component with the given name in the store.
func (s Store) Register(name string, c Component) (cancel func(), err error) {
	if c == nil {
		return nil, errors.Wrap(nilComponentErr, name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.components[name]; exists {
		return nil, errors.Wrap(existsErr, name)
	}

	s.components[name] = c
	s.notifyWaiters(name, c)

	return func() {
		s.mu.Lock()
		delete(s.components, name)
		s.mu.Unlock()
	}, nil
}

func (s Store) notifyWaiters(name string, c Component) {
	for waiter := range s.waiters[name] {
		waiter <- c
	}
}

func (s Store) unregister(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.components, name)
}

// Get gets a component from the store.
// If the component does not exist it returns nil.
func (s Store) Get(name string) Component {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.components[name]
}

func (s Store) addWaiter(name string, ch chan Component) {
	idx, exists := s.waiters[name]
	if !exists {
		idx = make(map[chan Component]struct{})
		s.waiters[name] = idx

	}
	idx[ch] = struct{}{}
}

func (s Store) removeWaiter(name string, ch chan Component) {
	delete(s.waiters[name], ch)
	if len(s.waiters) == 0 {
		delete(s.waiters, name)
	}
}

// Wait waits for a component with the given name to be added to the store.
// If the component already exists it is returned immediately.
// Wait only returns nil if the passed in context is cancelled.
func (s Store) Wait(ctx context.Context, name string) Component {
	s.mu.Lock()
	if c := s.components[name]; c != nil {
		s.mu.Unlock()
		return c
	}

	wait := make(chan Component, 1)

	s.addWaiter(name, wait)
	defer func() {
		s.mu.Lock()
		s.removeWaiter(name, wait)
		s.mu.Unlock()
	}()

	s.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil
	case c := <-wait:
		return c
	}
}
