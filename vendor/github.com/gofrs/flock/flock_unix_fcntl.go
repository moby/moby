// Copyright 2015 Tim Heckman. All rights reserved.
// Copyright 2018-2025 The Gofrs. All rights reserved.
// Use of this source code is governed by the BSD 3-Clause
// license that can be found in the LICENSE file.

// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This code implements the filelock API using POSIX 'fcntl' locks,
// which attach to an (inode, process) pair rather than a file descriptor.
// To avoid unlocking files prematurely when the same file is opened through different descriptors,
// we allow only one read-lock at a time.
//
// This code is adapted from the Go package (go.22):
// https://github.com/golang/go/blob/release-branch.go1.22/src/cmd/go/internal/lockedfile/internal/filelock/filelock_fcntl.go

//go:build aix || (solaris && !illumos)

package flock

import (
	"errors"
	"io"
	"io/fs"
	"math/rand"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// https://github.com/golang/go/blob/09aeb6e33ab426eff4676a3baf694d5a3019e9fc/src/cmd/go/internal/lockedfile/internal/filelock/filelock_fcntl.go#L28
type lockType int16

// String returns the name of the function corresponding to lt
// (Lock, RLock, or Unlock).
// https://github.com/golang/go/blob/09aeb6e33ab426eff4676a3baf694d5a3019e9fc/src/cmd/go/internal/lockedfile/internal/filelock/filelock.go#L67
func (lt lockType) String() string {
	switch lt {
	case readLock:
		return "RLock"
	case writeLock:
		return "Lock"
	default:
		return "Unlock"
	}
}

// https://github.com/golang/go/blob/09aeb6e33ab426eff4676a3baf694d5a3019e9fc/src/cmd/go/internal/lockedfile/internal/filelock/filelock_fcntl.go#L30-L33
const (
	readLock  lockType = unix.F_RDLCK
	writeLock lockType = unix.F_WRLCK
)

// https://github.com/golang/go/blob/09aeb6e33ab426eff4676a3baf694d5a3019e9fc/src/cmd/go/internal/lockedfile/internal/filelock/filelock_fcntl.go#L35
type inode = uint64

// https://github.com/golang/go/blob/09aeb6e33ab426eff4676a3baf694d5a3019e9fc/src/cmd/go/internal/lockedfile/internal/filelock/filelock_fcntl.go#L37-L40
type inodeLock struct {
	owner *Flock
	queue []<-chan *Flock
}

type cmdType int

const (
	tryLock  cmdType = unix.F_SETLK
	waitLock cmdType = unix.F_SETLKW
)

var (
	mu     sync.Mutex
	inodes = map[*Flock]inode{}
	locks  = map[inode]inodeLock{}
)

// Lock is a blocking call to try and take an exclusive file lock.
// It will wait until it is able to obtain the exclusive file lock.
// It's recommended that TryLock() be used over this function.
// This function may block the ability to query the current Locked() or RLocked() status due to a RW-mutex lock.
//
// If we are already exclusive-locked, this function short-circuits and
// returns immediately assuming it can take the mutex lock.
//
// If the *Flock has a shared lock (RLock),
// this may transparently replace the shared lock with an exclusive lock on some UNIX-like operating systems.
// Be careful when using exclusive locks in conjunction with shared locks (RLock()),
// because calling Unlock() may accidentally release the exclusive lock that was once a shared lock.
func (f *Flock) Lock() error {
	return f.lock(&f.l, writeLock)
}

// RLock is a blocking call to try and take a shared file lock.
// It will wait until it is able to obtain the shared file lock.
// It's recommended that TryRLock() be used over this function.
// This function may block the ability to query the current Locked() or RLocked() status due to a RW-mutex lock.
//
// If we are already shared-locked, this function short-circuits and
// returns immediately assuming it can take the mutex lock.
func (f *Flock) RLock() error {
	return f.lock(&f.r, readLock)
}

func (f *Flock) lock(locked *bool, flag lockType) error {
	f.m.Lock()
	defer f.m.Unlock()

	if *locked {
		return nil
	}

	if f.fh == nil {
		if err := f.setFh(f.flag); err != nil {
			return err
		}

		defer f.ensureFhState()
	}

	_, err := f.doLock(waitLock, flag, true)
	if err != nil {
		return err
	}

	*locked = true

	return nil
}

// https://github.com/golang/go/blob/09aeb6e33ab426eff4676a3baf694d5a3019e9fc/src/cmd/go/internal/lockedfile/internal/filelock/filelock_fcntl.go#L48
func (f *Flock) doLock(cmd cmdType, lt lockType, blocking bool) (bool, error) {
	// POSIX locks apply per inode and process,
	// and the lock for an inode is released when *any* descriptor for that inode is closed.
	// So we need to synchronize access to each inode internally,
	// and must serialize lock and unlock calls that refer to the same inode through different descriptors.
	fi, err := f.fh.Stat()
	if err != nil {
		return false, err
	}

	// Note(ldez): don't replace `syscall.Stat_t` by `unix.Stat_t` because `FileInfo.Sys()` returns `syscall.Stat_t`
	ino := fi.Sys().(*syscall.Stat_t).Ino

	mu.Lock()

	if i, dup := inodes[f]; dup && i != ino {
		mu.Unlock()
		return false, &fs.PathError{
			Op:   lt.String(),
			Path: f.Path(),
			Err:  errors.New("inode for file changed since last Lock or RLock"),
		}
	}

	inodes[f] = ino

	var wait chan *Flock

	l := locks[ino]

	switch {
	case l.owner == f:
		// This file already owns the lock, but the call may change its lock type.
	case l.owner == nil:
		// No owner: it's ours now.
		l.owner = f

	case !blocking:
		// Already owned: cannot take the lock.
		mu.Unlock()
		return false, nil

	default:
		// Already owned: add a channel to wait on.
		wait = make(chan *Flock)
		l.queue = append(l.queue, wait)
	}

	locks[ino] = l

	mu.Unlock()

	if wait != nil {
		wait <- f
	}

	// Spurious EDEADLK errors arise on platforms that compute deadlock graphs at
	// the process, rather than thread, level. Consider processes P and Q, with
	// threads P.1, P.2, and Q.3. The following trace is NOT a deadlock, but will be
	// reported as a deadlock on systems that consider only process granularity:
	//
	// 	P.1 locks file A.
	// 	Q.3 locks file B.
	// 	Q.3 blocks on file A.
	// 	P.2 blocks on file B. (This is erroneously reported as a deadlock.)
	// 	P.1 unlocks file A.
	// 	Q.3 unblocks and locks file A.
	// 	Q.3 unlocks files A and B.
	// 	P.2 unblocks and locks file B.
	// 	P.2 unlocks file B.
	//
	// These spurious errors were observed in practice on AIX and Solaris in
	// cmd/go: see https://golang.org/issue/32817.
	//
	// We work around this bug by treating EDEADLK as always spurious. If there
	// really is a lock-ordering bug between the interacting processes, it will
	// become a livelock instead, but that's not appreciably worse than if we had
	// a proper flock implementation (which generally does not even attempt to
	// diagnose deadlocks).
	//
	// In the above example, that changes the trace to:
	//
	// 	P.1 locks file A.
	// 	Q.3 locks file B.
	// 	Q.3 blocks on file A.
	// 	P.2 spuriously fails to lock file B and goes to sleep.
	// 	P.1 unlocks file A.
	// 	Q.3 unblocks and locks file A.
	// 	Q.3 unlocks files A and B.
	// 	P.2 wakes up and locks file B.
	// 	P.2 unlocks file B.
	//
	// We know that the retry loop will not introduce a *spurious* livelock
	// because, according to the POSIX specification, EDEADLK is only to be
	// returned when “the lock is blocked by a lock from another process”.
	// If that process is blocked on some lock that we are holding, then the
	// resulting livelock is due to a real deadlock (and would manifest as such
	// when using, for example, the flock implementation of this package).
	// If the other process is *not* blocked on some other lock that we are
	// holding, then it will eventually release the requested lock.

	nextSleep := 1 * time.Millisecond
	const maxSleep = 500 * time.Millisecond
	for {
		err = setlkw(f.fh.Fd(), cmd, lt)
		if !errors.Is(err, unix.EDEADLK) {
			break
		}

		time.Sleep(nextSleep)

		nextSleep += nextSleep
		if nextSleep > maxSleep {
			nextSleep = maxSleep
		}
		// Apply 10% jitter to avoid synchronizing collisions when we finally unblock.
		nextSleep += time.Duration((0.1*rand.Float64() - 0.05) * float64(nextSleep))
	}

	if err != nil {
		f.doUnlock()

		if cmd == tryLock && errors.Is(err, unix.EACCES) {
			return false, nil
		}

		return false, &fs.PathError{
			Op:   lt.String(),
			Path: f.Path(),
			Err:  err,
		}
	}

	return true, nil
}

func (f *Flock) Unlock() error {
	f.m.Lock()
	defer f.m.Unlock()

	// If we aren't locked or if the lockfile instance is nil
	// just return a nil error because we are unlocked.
	if (!f.l && !f.r) || f.fh == nil {
		return nil
	}

	if err := f.doUnlock(); err != nil {
		return err
	}

	f.reset()

	return nil
}

// https://github.com/golang/go/blob/09aeb6e33ab426eff4676a3baf694d5a3019e9fc/src/cmd/go/internal/lockedfile/internal/filelock/filelock_fcntl.go#L163
func (f *Flock) doUnlock() (err error) {
	var owner *Flock

	mu.Lock()

	ino, ok := inodes[f]
	if ok {
		owner = locks[ino].owner
	}

	mu.Unlock()

	if owner == f {
		err = setlkw(f.fh.Fd(), waitLock, unix.F_UNLCK)
	}

	mu.Lock()

	l := locks[ino]

	if len(l.queue) == 0 {
		// No waiters: remove the map entry.
		delete(locks, ino)
	} else {
		// The first waiter is sending us their file now.
		// Receive it and update the queue.
		l.owner = <-l.queue[0]
		l.queue = l.queue[1:]
		locks[ino] = l
	}

	delete(inodes, f)

	mu.Unlock()

	return err
}

// TryLock is the preferred function for taking an exclusive file lock.
// This function takes an RW-mutex lock before it tries to lock the file,
// so there is the possibility that this function may block for a short time
// if another goroutine is trying to take any action.
//
// The actual file lock is non-blocking.
// If we are unable to get the exclusive file lock,
// the function will return false instead of waiting for the lock.
// If we get the lock, we also set the *Flock instance as being exclusive-locked.
func (f *Flock) TryLock() (bool, error) {
	return f.try(&f.l, writeLock)
}

// TryRLock is the preferred function for taking a shared file lock.
// This function takes an RW-mutex lock before it tries to lock the file,
// so there is the possibility that this function may block for a short time
// if another goroutine is trying to take any action.
//
// The actual file lock is non-blocking.
// If we are unable to get the shared file lock,
// the function will return false instead of waiting for the lock.
// If we get the lock, we also set the *Flock instance as being share-locked.
func (f *Flock) TryRLock() (bool, error) {
	return f.try(&f.r, readLock)
}

func (f *Flock) try(locked *bool, flag lockType) (bool, error) {
	f.m.Lock()
	defer f.m.Unlock()

	if *locked {
		return true, nil
	}

	if f.fh == nil {
		if err := f.setFh(f.flag); err != nil {
			return false, err
		}

		defer f.ensureFhState()
	}

	hasLock, err := f.doLock(tryLock, flag, false)
	if err != nil {
		return false, err
	}

	*locked = hasLock

	return hasLock, nil
}

// setlkw calls FcntlFlock with cmd for the entire file indicated by fd.
// https://github.com/golang/go/blob/09aeb6e33ab426eff4676a3baf694d5a3019e9fc/src/cmd/go/internal/lockedfile/internal/filelock/filelock_fcntl.go#L198
func setlkw(fd uintptr, cmd cmdType, lt lockType) error {
	for {
		err := unix.FcntlFlock(fd, int(cmd), &unix.Flock_t{
			Type:   int16(lt),
			Whence: io.SeekStart,
			Start:  0,
			Len:    0, // All bytes.
		})
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}
