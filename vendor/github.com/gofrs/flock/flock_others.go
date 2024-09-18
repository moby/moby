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
