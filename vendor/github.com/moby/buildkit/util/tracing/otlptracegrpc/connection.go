// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlptracegrpc

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Connection struct {
	// Ensure pointer is 64-bit aligned for atomic operations on both 32 and 64 bit machines.
	lastConnectErrPtr unsafe.Pointer

	// mu protects the Connection as it is accessed by the
	// exporter goroutines and background Connection goroutine
	mu sync.Mutex
	cc *grpc.ClientConn

	// these fields are read-only after constructor is finished
	metadata             metadata.MD
	newConnectionHandler func(cc *grpc.ClientConn)

	// these channels are created once
	disconnectedCh             chan bool
	backgroundConnectionDoneCh chan struct{}
	stopCh                     chan struct{}

	// this is for tests, so they can replace the closing
	// routine without a worry of modifying some global variable
	// or changing it back to original after the test is done
	closeBackgroundConnectionDoneCh func(ch chan struct{})
}

func NewConnection(cc *grpc.ClientConn, handler func(cc *grpc.ClientConn)) *Connection {
	c := new(Connection)
	c.newConnectionHandler = handler
	c.cc = cc
	c.closeBackgroundConnectionDoneCh = func(ch chan struct{}) {
		close(ch)
	}
	return c
}

func (c *Connection) StartConnection(ctx context.Context) error {
	c.stopCh = make(chan struct{})
	c.disconnectedCh = make(chan bool, 1)
	c.backgroundConnectionDoneCh = make(chan struct{})

	if err := c.connect(); err == nil {
		c.setStateConnected()
	} else {
		c.SetStateDisconnected(err)
	}
	go c.indefiniteBackgroundConnection()

	// TODO: proper error handling when initializing connections.
	// We can report permanent errors, e.g., invalid settings.
	return nil
}

func (c *Connection) LastConnectError() error {
	errPtr := (*error)(atomic.LoadPointer(&c.lastConnectErrPtr))
	if errPtr == nil {
		return nil
	}
	return *errPtr
}

func (c *Connection) saveLastConnectError(err error) {
	var errPtr *error
	if err != nil {
		errPtr = &err
	}
	atomic.StorePointer(&c.lastConnectErrPtr, unsafe.Pointer(errPtr))
}

func (c *Connection) SetStateDisconnected(err error) {
	c.saveLastConnectError(err)
	select {
	case c.disconnectedCh <- true:
	default:
	}
	c.newConnectionHandler(nil)
}

func (c *Connection) setStateConnected() {
	c.saveLastConnectError(nil)
}

func (c *Connection) Connected() bool {
	return c.LastConnectError() == nil
}

const defaultConnReattemptPeriod = 10 * time.Second

func (c *Connection) indefiniteBackgroundConnection() {
	defer func() {
		c.closeBackgroundConnectionDoneCh(c.backgroundConnectionDoneCh)
	}()

	connReattemptPeriod := defaultConnReattemptPeriod

	rng := rand.New(rand.NewSource(time.Now().UnixNano() + rand.Int63n(1024))) //nolint:gosec // No strong seeding required, nano time can already help with pseudo uniqueness.

	// maxJitterNanos: 70% of the connectionReattemptPeriod
	maxJitterNanos := int64(0.7 * float64(connReattemptPeriod))

	for {
		// Otherwise these will be the normal scenarios to enable
		// reconnection if we trip out.
		// 1. If we've stopped, return entirely
		// 2. Otherwise block until we are disconnected, and
		//    then retry connecting
		select {
		case <-c.stopCh:
			return

		case <-c.disconnectedCh:
			// Quickly check if we haven't stopped at the
			// same time.
			select {
			case <-c.stopCh:
				return

			default:
			}

			// Normal scenario that we'll wait for
		}

		if err := c.connect(); err == nil {
			c.setStateConnected()
		} else {
			// this code is unreachable in most cases
			// c.connect does not establish Connection
			c.SetStateDisconnected(err)
		}

		// Apply some jitter to avoid lockstep retrials of other
		// collector-exporters. Lockstep retrials could result in an
		// innocent DDOS, by clogging the machine's resources and network.
		jitter := time.Duration(rng.Int63n(maxJitterNanos))
		select {
		case <-c.stopCh:
			return
		case <-time.After(connReattemptPeriod + jitter):
		}
	}
}

func (c *Connection) connect() error {
	c.newConnectionHandler(c.cc)
	return nil
}

func (c *Connection) ContextWithMetadata(ctx context.Context) context.Context {
	if c.metadata.Len() > 0 {
		return metadata.NewOutgoingContext(ctx, c.metadata)
	}
	return ctx
}

func (c *Connection) Shutdown(ctx context.Context) error {
	close(c.stopCh)
	// Ensure that the backgroundConnector returns
	select {
	case <-c.backgroundConnectionDoneCh:
	case <-ctx.Done():
		return context.Cause(ctx)
	}

	c.mu.Lock()
	cc := c.cc
	c.cc = nil
	c.mu.Unlock()

	if cc != nil {
		return cc.Close()
	}

	return nil
}

func (c *Connection) ContextWithStop(ctx context.Context) (context.Context, context.CancelCauseFunc) {
	// Unify the parent context Done signal with the Connection's
	// stop channel.
	ctx, cancel := context.WithCancelCause(ctx)
	go func(ctx context.Context, cancel context.CancelCauseFunc) {
		select {
		case <-ctx.Done():
			// Nothing to do, either cancelled or deadline
			// happened.
		case <-c.stopCh:
			cancel(errors.WithStack(context.Canceled))
		}
	}(ctx, cancel)
	return ctx, cancel
}
