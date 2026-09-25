package registrar

import (
	"context"
	"sync"
	"time"
)

type Registrar[K comparable, V any] struct {
	mu     sync.Mutex
	values map[K]*registrarValue[V]
}

func New[K comparable, V any]() *Registrar[K, V] {
	return &Registrar[K, V]{
		values: make(map[K]*registrarValue[V]),
	}
}

// Register will register the value with the given id.
// This value will persist until Discard is called with the same id.
func (r *Registrar[K, V]) Register(id K, val V) {
	reg := r.getOrCreateRegistrar(id, nil)
	reg.Register(val, nil)
}

// Get will retrieve a registered value and will wait a small time period for that
// value to appear if it hasn't been registered yet.
func (r *Registrar[K, V]) Get(ctx context.Context, id K) (v V, _ error) {
	onCreate := func(reg *registrarValue[V]) {
		select {
		case <-reg.notifyCh:
			return
		case <-time.After(3 * time.Second):
			r.Discard(id)
		}
	}

	reg := r.getOrCreateRegistrar(id, onCreate)

	select {
	case <-ctx.Done():
		return v, context.Cause(ctx)
	case <-reg.notifyCh:
		return reg.value, reg.err
	}
}

// Discard will remove the given value from the registrar after it has been registered
// with Register.
func (r *Registrar[K, V]) Discard(id K) {
	r.mu.Lock()
	reg, ok := r.values[id]
	delete(r.values, id)
	r.mu.Unlock()

	if ok {
		var value V
		reg.Register(value, context.Canceled)
	}
}

// getOrCreateRegistrar will create a registrar with the given id to be retrieved at a later time.
// The same id will return the same registrar.
//
// If the registrar is newly created, the onCreate function is invoked in a separate goroutine
// if it is present. If nil, this function is ignored.
func (r *Registrar[K, V]) getOrCreateRegistrar(id K, onCreate func(*registrarValue[V])) *registrarValue[V] {
	r.mu.Lock()
	defer r.mu.Unlock()

	reg, ok := r.values[id]
	if !ok {
		reg = &registrarValue[V]{
			notifyCh: make(chan struct{}),
		}
		r.values[id] = reg

		if onCreate != nil {
			go onCreate(reg)
		}
	}
	return reg
}

type registrarValue[V any] struct {
	// notifyCh is the notification channel that gets closed when
	// the bridge is registered.
	notifyCh chan struct{}

	value V
	err   error
	isSet bool

	mu sync.Mutex
}

func (r *registrarValue[V]) Register(value V, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isSet {
		return
	}

	r.value = value
	r.err = err
	r.isSet = true
	close(r.notifyCh)
}
