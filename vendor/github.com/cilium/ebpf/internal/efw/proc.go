//go:build windows

package efw

import (
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
)

/*
The BPF syscall wrapper which is ABI compatible with Linux.

	int bpf(int cmd, union bpf_attr* attr, unsigned int size)
*/
var BPF = newProc("bpf")

type proc struct {
	proc *windows.LazyProc
}

func newProc(name string) proc {
	return proc{module.NewProc(name)}
}

func (p proc) Find() (uintptr, error) {
	if err := p.proc.Find(); err != nil {
		if errors.Is(err, windows.ERROR_MOD_NOT_FOUND) {
			return 0, fmt.Errorf("load %s: not found", module.Name)
		}
		return 0, err
	}
	return p.proc.Addr(), nil
}

// uint32Result wraps a function which returns a uint32_t.
func uint32Result(r1, _ uintptr, _ syscall.Errno) uint32 {
	return uint32(r1)
}

// errorResult wraps a function which returns ebpf_result_t.
func errorResult(r1, _ uintptr, errNo syscall.Errno) error {
	err := resultToError(Result(r1))
	if err != nil && errNo != 0 {
		return fmt.Errorf("%w (errno: %v)", err, errNo)
	}
	return err
}
