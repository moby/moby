// Copyright 2015 Tim Heckman. All rights reserved.
// Copyright 2018-2025 The Gofrs. All rights reserved.
// Use of this source code is governed by the BSD 3-Clause
// license that can be found in the LICENSE file.

//go:build windows

package flock

import (
	"errors"

	"golang.org/x/sys/windows"
)

// Use of 0x00000000 for the shared lock is a guess based on some the MS Windows `LockFileEX` docs,
// which document the `LOCKFILE_EXCLUSIVE_LOCK` flag as:
//
// > The function requests an exclusive lock. Otherwise, it requests a shared lock.
//
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa365203(v=vs.85).aspx
const winLockfileSharedLock = 0x00000000

// ErrorLockViolation is the error code returned from the Windows syscall when a lock would block,
// and you ask to fail immediately.
//
//nolint:errname // It should be renamed to `ErrLockViolation`.
const ErrorLockViolation windows.Errno = 0x21 // 33

// Lock is a blocking call to try and take an exclusive file lock.
// It will wait until it is able to obtain the exclusive file lock.
// It's recommended that TryLock() be used over this function.
// This function may block the ability to query the current Locked() or RLocked() status due to a RW-mutex lock.
//
// If we are already locked, this function short-circuits and
// returns immediately assuming it can take the mutex lock.
func (f *Flock) Lock() error {
	return f.lock(&f.l, windows.LOCKFILE_EXCLUSIVE_LOCK)
}

// RLock is a blocking call to try and take a shared file lock.
// It will wait until it is able to obtain the shared file lock.
// It's recommended that TryRLock() be used over this function.
// This function may block the ability to query the current Locked() or RLocked() status due to a RW-mutex lock.
//
// If we are already locked, this function short-circuits and
// returns immediately assuming it can take the mutex lock.
func (f *Flock) RLock() error {
	return f.lock(&f.r, winLockfileSharedLock)
}

func (f *Flock) lock(locked *bool, flag uint32) error {
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

	err := windows.LockFileEx(windows.Handle(f.fh.Fd()), flag, 0, 1, 0, &windows.Overlapped{})
	if err != nil && !errors.Is(err, windows.Errno(0)) {
		return err
	}

	*locked = true

	return nil
}

// Unlock is a function to unlock the file.
// This file takes a RW-mutex lock,
// so while it is running the Locked() and RLocked() functions will be blocked.
//
// This function short-circuits if we are unlocked already.
// If not, it calls UnlockFileEx() on the file and closes the file descriptor.
// It does not remove the file from disk.
// It's up to your application to do.
func (f *Flock) Unlock() error {
	f.m.Lock()
	defer f.m.Unlock()

	// if we aren't locked or if the lockfile instance is nil
	// just return a nil error because we are unlocked
	if (!f.l && !f.r) || f.fh == nil {
		return nil
	}

	// mark the file as unlocked
	err := windows.UnlockFileEx(windows.Handle(f.fh.Fd()), 0, 1, 0, &windows.Overlapped{})
	if err != nil && !errors.Is(err, windows.Errno(0)) {
		return err
	}

	f.reset()

	return nil
}

// TryLock is the preferred function for taking an exclusive file lock.
// This function does take a RW-mutex lock before it tries to lock the file,
// so there is the possibility that this function may block for a short time
// if another goroutine is trying to take any action.
//
// The actual file lock is non-blocking.
// If we are unable to get the exclusive file lock,
// the function will return false instead of waiting for the lock.
// If we get the lock, we also set the *Flock instance as being exclusive-locked.
func (f *Flock) TryLock() (bool, error) {
	return f.try(&f.l, windows.LOCKFILE_EXCLUSIVE_LOCK)
}

// TryRLock is the preferred function for taking a shared file lock.
// This function does take a RW-mutex lock before it tries to lock the file,
// so there is the possibility that this function may block for a short time if another goroutine is trying to take any action.
//
// The actual file lock is non-blocking.
// If we are unable to get the shared file lock,
// the function will return false instead of waiting for the lock.
// If we get the lock, we also set the *Flock instance as being shared-locked.
func (f *Flock) TryRLock() (bool, error) {
	return f.try(&f.r, winLockfileSharedLock)
}

func (f *Flock) try(locked *bool, flag uint32) (bool, error) {
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

	err := windows.LockFileEx(windows.Handle(f.fh.Fd()), flag|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
	if err != nil && !errors.Is(err, windows.Errno(0)) {
		if errors.Is(err, ErrorLockViolation) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return false, nil
		}

		return false, err
	}

	*locked = true

	return true, nil
}
