// Copyright 2015 Tim Heckman. All rights reserved.
// Copyright 2018-2024 The Gofrs. All rights reserved.
// Use of this source code is governed by the BSD 3-Clause
// license that can be found in the LICENSE file.

// Package flock implements a thread-safe interface for file locking.
// It also includes a non-blocking TryLock() function to allow locking
// without blocking execution.
//
// Package flock is released under the BSD 3-Clause License. See the LICENSE file
// for more details.
//
// While using this library, remember that the locking behaviors are not
// guaranteed to be the same on each platform. For example, some UNIX-like
// operating systems will transparently convert a shared lock to an exclusive
// lock. If you Unlock() the flock from a location where you believe that you
// have the shared lock, you may accidentally drop the exclusive lock.
package flock

import (
	"context"
	"io/fs"
	"os"
	"runtime"
	"sync"
	"time"
)

type Option func(f *Flock)

// SetFlag sets the flag used to create/open the file.
func SetFlag(flag int) Option {
	return func(f *Flock) {
		f.flag = flag
	}
}

// SetPermissions sets the OS permissions to set on the file.
func SetPermissions(perm fs.FileMode) Option {
	return func(f *Flock) {
		f.perm = perm
	}
}

// Flock is the struct type to handle file locking. All fields are unexported,
// with access to some of the fields provided by getter methods (Path() and Locked()).
type Flock struct {
	path string
	m    sync.RWMutex
	fh   *os.File
	l    bool
	r    bool

	// flag is the flag used to create/open the file.
	flag int
	// perm is the OS permissions to set on the file.
	perm fs.FileMode
}

// New returns a new instance of *Flock. The only parameter
// it takes is the path to the desired lockfile.
func New(path string, opts ...Option) *Flock {
	// create it if it doesn't exist, and open the file read-only.
	flags := os.O_CREATE
	switch runtime.GOOS {
	case "aix", "solaris", "illumos":
		// AIX cannot preform write-lock (i.e. exclusive) on a read-only file.
		flags |= os.O_RDWR
	default:
		flags |= os.O_RDONLY
	}

	f := &Flock{
		path: path,
		flag: flags,
		perm: fs.FileMode(0o600),
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

// NewFlock returns a new instance of *Flock. The only parameter
// it takes is the path to the desired lockfile.
//
// Deprecated: Use New instead.
func NewFlock(path string) *Flock {
	return New(path)
}

// Close is equivalent to calling Unlock.
//
// This will release the lock and close the underlying file descriptor.
// It will not remove the file from disk, that's up to your application.
func (f *Flock) Close() error {
	return f.Unlock()
}

// Path returns the path as provided in NewFlock().
func (f *Flock) Path() string {
	return f.path
}

// Locked returns the lock state (locked: true, unlocked: false).
//
// Warning: by the time you use the returned value, the state may have changed.
func (f *Flock) Locked() bool {
	f.m.RLock()
	defer f.m.RUnlock()

	return f.l
}

// RLocked returns the read lock state (locked: true, unlocked: false).
//
// Warning: by the time you use the returned value, the state may have changed.
func (f *Flock) RLocked() bool {
	f.m.RLock()
	defer f.m.RUnlock()

	return f.r
}

func (f *Flock) String() string {
	return f.path
}

// TryLockContext repeatedly tries to take an exclusive lock until one of the conditions is met:
// - TryLock succeeds
// - TryLock fails with error
// - Context Done channel is closed.
func (f *Flock) TryLockContext(ctx context.Context, retryDelay time.Duration) (bool, error) {
	return tryCtx(ctx, f.TryLock, retryDelay)
}

// TryRLockContext repeatedly tries to take a shared lock until one of the conditions is met:
// - TryRLock succeeds
// - TryRLock fails with error
// - Context Done channel is closed.
func (f *Flock) TryRLockContext(ctx context.Context, retryDelay time.Duration) (bool, error) {
	return tryCtx(ctx, f.TryRLock, retryDelay)
}

func tryCtx(ctx context.Context, fn func() (bool, error), retryDelay time.Duration) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	for {
		if ok, err := fn(); ok || err != nil {
			return ok, err
		}

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(retryDelay):
			// try again
		}
	}
}

func (f *Flock) setFh() error {
	// open a new os.File instance
	fh, err := os.OpenFile(f.path, f.flag, f.perm)
	if err != nil {
		return err
	}

	// set the file handle on the struct
	f.fh = fh

	return nil
}

// ensure the file handle is closed if no lock is held.
func (f *Flock) ensureFhState() {
	if f.l || f.r || f.fh == nil {
		return
	}

	_ = f.fh.Close()

	f.fh = nil
}

func (f *Flock) reset() {
	f.l = false
	f.r = false

	_ = f.fh.Close()

	f.fh = nil
}
