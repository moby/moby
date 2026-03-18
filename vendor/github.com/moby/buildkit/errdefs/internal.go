package errdefs

import (
	"errors"
	"syscall"
)

type internalError struct {
	error
}

func (internalError) System() {}

func (err internalError) Unwrap() error {
	return err.error
}

type system interface {
	System()
}

var _ system = internalError{}

func Internal(err error) error {
	if err == nil {
		return nil
	}
	return internalError{err}
}

func IsInternal(err error) bool {
	var s system
	if errors.As(err, &s) {
		return true
	}

	var errno syscall.Errno
	if errors.As(err, &errno) {
		if _, ok := isInternalSyscall(errno); ok {
			return true
		}
	}
	return false
}

func IsResourceExhausted(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if v, ok := isInternalSyscall(errno); ok && v {
			return v
		}
	}
	return false
}

func isInternalSyscall(err syscall.Errno) (bool, bool) {
	v, ok := syscallErrors()[err]
	return v, ok
}
