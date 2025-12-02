//go:build !tinygo

package sysfs

import (
	"syscall"
	"time"
	"unsafe"

	"github.com/tetratelabs/wazero/experimental/sys"
)

// pollFd is the struct to query for file descriptor events using poll.
type pollFd struct {
	// fd is the file descriptor.
	fd int32
	// events is a bitmap containing the requested events.
	events int16
	// revents is a bitmap containing the returned events.
	revents int16
}

// newPollFd is a constructor for pollFd that abstracts the platform-specific type of file descriptors.
func newPollFd(fd uintptr, events, revents int16) pollFd {
	return pollFd{fd: int32(fd), events: events, revents: revents}
}

// _POLLIN subscribes a notification when any readable data is available.
const _POLLIN = 0x0001

// _poll implements poll on Linux via ppoll.
func _poll(fds []pollFd, timeoutMillis int32) (n int, errno sys.Errno) {
	var ts syscall.Timespec
	if timeoutMillis >= 0 {
		ts = syscall.NsecToTimespec(int64(time.Duration(timeoutMillis) * time.Millisecond))
	}
	return ppoll(fds, &ts)
}

// ppoll is a poll variant that allows to subscribe to a mask of signals.
// However, we do not need such mask, so the corresponding argument is always nil.
func ppoll(fds []pollFd, timespec *syscall.Timespec) (n int, err sys.Errno) {
	var fdptr *pollFd
	nfd := len(fds)
	if nfd != 0 {
		fdptr = &fds[0]
	}

	n1, _, errno := syscall.Syscall6(
		uintptr(syscall.SYS_PPOLL),
		uintptr(unsafe.Pointer(fdptr)),
		uintptr(nfd),
		uintptr(unsafe.Pointer(timespec)),
		uintptr(unsafe.Pointer(nil)), // sigmask is currently always ignored
		uintptr(unsafe.Pointer(nil)),
		uintptr(unsafe.Pointer(nil)))

	return int(n1), sys.UnwrapOSError(errno)
}
