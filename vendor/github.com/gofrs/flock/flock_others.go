// Copyright 2015 Tim Heckman. All rights reserved.
// Copyright 2018-2025 The Gofrs. All rights reserved.
// Use of this source code is governed by the BSD 3-Clause
// license that can be found in the LICENSE file.

//go:build (!unix && !windows) || plan9

package flock

import (
	"errors"
	"io/fs"
)

func (f *Flock) Lock() error {
	return &fs.PathError{
		Op:   "Lock",
		Path: f.Path(),
		Err:  errors.ErrUnsupported,
	}
}

func (f *Flock) RLock() error {
	return &fs.PathError{
		Op:   "RLock",
		Path: f.Path(),
		Err:  errors.ErrUnsupported,
	}
}

func (f *Flock) Unlock() error {
	return &fs.PathError{
		Op:   "Unlock",
		Path: f.Path(),
		Err:  errors.ErrUnsupported,
	}
}

func (f *Flock) TryLock() (bool, error) {
	return false, f.Lock()
}

func (f *Flock) TryRLock() (bool, error) {
	return false, f.RLock()
}
