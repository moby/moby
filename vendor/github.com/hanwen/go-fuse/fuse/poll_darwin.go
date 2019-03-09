package fuse

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

type pollFd struct {
	Fd      int32
	Events  int16
	Revents int16
}

func sysPoll(fds []pollFd, timeout int) (n int, err error) {
	r0, _, e1 := syscall.Syscall(syscall.SYS_POLL, uintptr(unsafe.Pointer(&fds[0])),
		uintptr(len(fds)), uintptr(timeout))
	n = int(r0)
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return n, err
}

func pollHack(mountPoint string) error {
	const (
		POLLIN    = 0x1
		POLLPRI   = 0x2
		POLLOUT   = 0x4
		POLLRDHUP = 0x2000
		POLLERR   = 0x8
		POLLHUP   = 0x10
	)

	fd, err := syscall.Open(filepath.Join(mountPoint, pollHackName), syscall.O_CREAT|syscall.O_TRUNC|syscall.O_RDWR, 0644)
	if err != nil {
		return err
	}
	pollData := []pollFd{{
		Fd:     int32(fd),
		Events: POLLIN | POLLPRI | POLLOUT,
	}}

	// Trigger _OP_POLL, so we can say ENOSYS. We don't care about
	// the return value.
	sysPoll(pollData, 0)
	syscall.Close(fd)
	return nil
}
