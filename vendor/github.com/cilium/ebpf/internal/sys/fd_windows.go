package sys

import (
	"fmt"
	"os"
	"runtime"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/efw"
)

// FD wraps a handle which is managed by the eBPF for Windows runtime.
//
// It is not equivalent to a real file descriptor or handle.
type FD struct {
	raw     int
	cleanup runtime.Cleanup
}

// NewFD wraps a raw fd with a finalizer.
//
// You must not use the raw fd after calling this function.
func NewFD(value int) (*FD, error) {
	if value == invalidFd {
		return nil, fmt.Errorf("invalid fd %d", value)
	}

	if value == 0 {
		// The efW runtime never uses zero fd it seems. No need to dup it.
		return nil, fmt.Errorf("invalid zero fd")
	}

	return newFD(value), nil
}

func (fd *FD) Close() error {
	if fd.raw == invalidFd {
		return nil
	}

	return efw.EbpfCloseFd(fd.Disown())
}

func (fd *FD) Dup() (*FD, error) {
	if fd.raw == invalidFd {
		return nil, ErrClosedFd
	}

	dup, err := efw.EbpfDuplicateFd(fd.raw)
	if err != nil {
		return nil, err
	}

	return NewFD(int(dup))
}

// File is not implemented.
func (fd *FD) File(name string) (*os.File, error) {
	return nil, fmt.Errorf("file from fd: %w", internal.ErrNotSupportedOnOS)
}
