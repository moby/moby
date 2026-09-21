//go:build windows

package efw

import (
	"syscall"
	"unsafe"
)

// ebpf_result_t ebpf_close_fd(fd_t fd)
var ebpfCloseFdProc = newProc("ebpf_close_fd")

func EbpfCloseFd(fd int) error {
	addr, err := ebpfCloseFdProc.Find()
	if err != nil {
		return err
	}

	return errorResult(syscall.SyscallN(addr, uintptr(fd)))
}

// ebpf_result_t ebpf_duplicate_fd(fd_t fd, _Out_ fd_t* dup)
var ebpfDuplicateFdProc = newProc("ebpf_duplicate_fd")

func EbpfDuplicateFd(fd int) (int, error) {
	addr, err := ebpfDuplicateFdProc.Find()
	if err != nil {
		return -1, err
	}

	var dup FD
	err = errorResult(syscall.SyscallN(addr, uintptr(fd), uintptr(unsafe.Pointer(&dup))))
	return int(dup), err
}
