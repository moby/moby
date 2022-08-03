/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package shutdown

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// ErrShutdown is the error condition when a context has been fully shutdown
var ErrShutdown = errors.New("shutdown")

// Service is used to facilitate shutdown by through callback
// registration and shutdown initiation
type Service interface {
	// Shutdown initiates shutdown
	Shutdown()
	// RegisterCallback registers functions to be called on shutdown and before
	// the shutdown channel is closed. A callback error will propagate to the
	// context error
	RegisterCallback(func(context.Context) error)
}

// WithShutdown returns a context which is similar to a cancel context, but
// with callbacks which can propagate to the context error. Unlike a cancel
// context, the shutdown context cannot be canceled from the parent context.
// However, future child contexes will be canceled upon shutdown.
func WithShutdown(ctx context.Context) (context.Context, Service) {
	ss := &shutdownService{
		Context: ctx,
		doneC:   make(chan struct{}),
		timeout: 30 * time.Second,
	}
	return ss, ss
}

type shutdownService struct {
	context.Context

	mu         sync.Mutex
	isShutdown bool
	callbacks  []func(context.Context) error
	doneC      chan struct{}
	err        error
	timeout    time.Duration
}

func (s *shutdownService) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isShutdown {
		return
	}
	s.isShutdown = true

	go func(callbacks []func(context.Context) error) {
		ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
		defer cancel()
		grp, ctx := errgroup.WithContext(ctx)
		for i := range callbacks {
			fn := callbacks[i]
			grp.Go(func() error { return fn(ctx) })
		}
		err := grp.Wait()
		if err == nil {
			err = ErrShutdown
		}
		s.mu.Lock()
		s.err = err
		close(s.doneC)
		s.mu.Unlock()
	}(s.callbacks)
}

func (s *shutdownService) Done() <-chan struct{} {
	return s.doneC
}

func (s *shutdownService) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}
func (s *shutdownService) RegisterCallback(fn func(context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.callbacks == nil {
		s.callbacks = []func(context.Context) error{}
	}
	s.callbacks = append(s.callbacks, fn)
}
