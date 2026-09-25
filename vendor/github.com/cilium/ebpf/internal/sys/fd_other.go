//go:build !windows

package sys

import (
	"fmt"
	"os"
	"runtime"

	"github.com/cilium/ebpf/internal/unix"
)

type FD struct {
	raw     int
	cleanup runtime.Cleanup
}

// NewFD wraps a raw fd with a finalizer.
//
// You must not use the raw fd after calling this function, since the underlying
// file descriptor number may change. This is because the BPF UAPI assumes that
// zero is not a valid fd value.
func NewFD(value int) (*FD, error) {
	if value < 0 {
		return nil, fmt.Errorf("invalid fd %d", value)
	}

	fd := newFD(value)
	if value != 0 {
		return fd, nil
	}

	dup, err := fd.Dup()
	_ = fd.Close()
	return dup, err
}

func (fd *FD) Close() error {
	if fd.raw < 0 {
		return nil
	}

	return unix.Close(fd.Disown())
}

func (fd *FD) Dup() (*FD, error) {
	if fd.raw < 0 {
		return nil, ErrClosedFd
	}

	// Always require the fd to be larger than zero: the BPF API treats the value
	// as "no argument provided".
	dup, err := unix.FcntlInt(uintptr(fd.raw), unix.F_DUPFD_CLOEXEC, 1)
	if err != nil {
		return nil, fmt.Errorf("can't dup fd: %v", err)
	}

	return newFD(dup), nil
}

// File takes ownership of FD and turns it into an [*os.File].
//
// You must not use the FD after the call returns.
//
// Returns [ErrClosedFd] if the fd is not valid.
func (fd *FD) File(name string) (*os.File, error) {
	if fd.raw == invalidFd {
		return nil, ErrClosedFd
	}

	return os.NewFile(uintptr(fd.Disown()), name), nil
}
