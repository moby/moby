package sysfs

import (
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

// _poll implements poll on Darwin via the corresponding libc function.
func _poll(fds []pollFd, timeoutMillis int32) (n int, errno sys.Errno) {
	var fdptr *pollFd
	nfds := len(fds)
	if nfds > 0 {
		fdptr = &fds[0]
	}
	n1, _, err := syscall_syscall6(
		libc_poll_trampoline_addr,
		uintptr(unsafe.Pointer(fdptr)),
		uintptr(nfds),
		uintptr(int(timeoutMillis)),
		uintptr(unsafe.Pointer(nil)),
		uintptr(unsafe.Pointer(nil)),
		uintptr(unsafe.Pointer(nil)))
	return int(n1), sys.UnwrapOSError(err)
}

// libc_poll_trampoline_addr is the address of the
// `libc_poll_trampoline` symbol, defined in `poll_darwin.s`.
//
// We use this to invoke the syscall through syscall_syscall6 imported below.
var libc_poll_trampoline_addr uintptr

// Imports the select symbol from libc as `libc_poll`.
//
// Note: CGO mechanisms are used in darwin regardless of the CGO_ENABLED value
// or the "cgo" build flag. See /RATIONALE.md for why.
//go:cgo_import_dynamic libc_poll poll "/usr/lib/libSystem.B.dylib"
