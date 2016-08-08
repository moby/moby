package fuse

import (
	"os"
	"syscall"
	"unsafe"
)

// TODO - move these into Go's syscall package.

func sys_writev(fd int, iovecs *syscall.Iovec, cnt int) (n int, err error) {
	n1, _, e1 := syscall.Syscall(
		syscall.SYS_WRITEV,
		uintptr(fd), uintptr(unsafe.Pointer(iovecs)), uintptr(cnt))
	n = int(n1)
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return n, err
}

func writev(fd int, packet [][]byte) (n int, err error) {
	iovecs := make([]syscall.Iovec, 0, len(packet))

	for _, v := range packet {
		if len(v) == 0 {
			continue
		}
		vec := syscall.Iovec{
			Base: &v[0],
		}
		vec.SetLen(len(v))
		iovecs = append(iovecs, vec)
	}

	sysErr := handleEINTR(func() error {
		var err error
		n, err = sys_writev(fd, &iovecs[0], len(iovecs))
		return err
	})
	if sysErr != nil {
		err = os.NewSyscallError("writev", sysErr)
	}
	return n, err
}
