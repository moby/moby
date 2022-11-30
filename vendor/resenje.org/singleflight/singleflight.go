// Copyright (c) 2019, Janoš Guljaš <janos@resenje.org>
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package singleflight provides a duplicate function call suppression
// mechanism similar to golang.org/x/sync/singleflight with support
// for context cancelation.
package singleflight

import (
	"context"
	"sync"
)

// Group represents a class of work and forms a namespace in
// which units of work can be executed with duplicate suppression.
type Group[K comparable, V any] struct {
	calls map[K]*call[V] // lazily initialized
	mu    sync.Mutex       // protects calls
}

// Do executes and returns the results of the given function, making sure that
// only one execution is in-flight for a given key at a time. If a duplicate
// comes in, the duplicate caller waits for the original to complete and
// receives the same results.
//
// The context passed to the fn function is a new context which is canceled when
// contexts from all callers are canceled, so that no caller is expecting the
// result. If there are multiple callers, context passed to one caller does not
// effect the execution and returned values of others.
//
// The return value shared indicates whether v was given to multiple callers.
func (g *Group[K, V]) Do(ctx context.Context, key K, fn func(ctx context.Context) (V, error)) (v V, shared bool, err error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[K]*call[V])
	}

	if c, ok := g.calls[key]; ok {
		c.shared = true
		c.counter++
		g.mu.Unlock()

		return g.wait(ctx, key, c)
	}

	callCtx, cancel := context.WithCancel(context.Background())

	c := &call[V]{
		done:    make(chan struct{}),
		cancel:  cancel,
		counter: 1,
	}
	g.calls[key] = c
	g.mu.Unlock()

	go func() {
		c.val, c.err = fn(callCtx)
		close(c.done)
	}()

	return g.wait(ctx, key, c)
}

// wait for function passed to Do to finish or context to be done.
func (g *Group[K, V]) wait(ctx context.Context, key K, c *call[V]) (v V, shared bool, err error) {
	select {
	case <-c.done:
		v = c.val
		err = c.err
	case <-ctx.Done():
		err = ctx.Err()
	}
	g.mu.Lock()
	c.counter--
	if c.counter == 0 {
		c.cancel()
	}
	if !c.forgotten {
		delete(g.calls, key)
	}
	g.mu.Unlock()
	return v, c.shared, err
}

// Forget tells the singleflight to forget about a key. Future calls
// to Do for this key will call the function rather than waiting for
// an earlier call to complete.
func (g *Group[K, V]) Forget(key K) {
	g.mu.Lock()
	if c, ok := g.calls[key]; ok {
		c.forgotten = true
	}
	delete(g.calls, key)
	g.mu.Unlock()
}

// call stores information about as single function call passed to Do function.
type call[V any] struct {
	// val and err hold the state about results of the function call.
	val V
	err error

	// done channel signals that the function call is done.
	done chan struct{}

	// forgotten indicates whether Forget was called with this call's key
	// while the call was still in flight.
	forgotten bool

	// shared indicates if results val and err are passed to multiple callers.
	shared bool

	// Number of callers that are waiting for the result.
	counter int
	// Cancel function for the context passed to the executing function.
	cancel context.CancelFunc
}
