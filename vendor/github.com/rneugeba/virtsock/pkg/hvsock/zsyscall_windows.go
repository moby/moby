package hvsock

import (
	"syscall"
	"unsafe"
)

var (
	modws2_32   = syscall.NewLazyDLL("ws2_32.dll")
	modwinmm    = syscall.NewLazyDLL("winmm.dll")
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")

	procConnect                            = modws2_32.NewProc("connect")
	procbind                               = modws2_32.NewProc("bind")
	procCancelIoEx                         = modkernel32.NewProc("CancelIoEx")
	procCreateIoCompletionPort             = modkernel32.NewProc("CreateIoCompletionPort")
	procGetQueuedCompletionStatus          = modkernel32.NewProc("GetQueuedCompletionStatus")
	procSetFileCompletionNotificationModes = modkernel32.NewProc("SetFileCompletionNotificationModes")
	proctimeBeginPeriod                    = modwinmm.NewProc("timeBeginPeriod")
)

// Do the interface allocations only once for common
// Errno values.
const (
	errnoERROR_IO_PENDING = 997
)

var (
	errERROR_IO_PENDING error = syscall.Errno(errnoERROR_IO_PENDING)
)

// errnoErr returns common boxed Errno values, to prevent
// allocations at runtime.
func errnoErr(e syscall.Errno) error {
	switch e {
	case 0:
		return nil
	case errnoERROR_IO_PENDING:
		return errERROR_IO_PENDING
	}
	// TODO: add more here, after collecting data on the common
	// error values see on Windows. (perhaps when running
	// all.bat?)
	return e
}

func sys_connect(s syscall.Handle, name unsafe.Pointer, namelen int32) (err error) {
	r1, _, e1 := syscall.Syscall(procConnect.Addr(), 3, uintptr(s), uintptr(name), uintptr(namelen))
	if r1 == socket_error {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}

	return
}

func sys_bind(s syscall.Handle, name unsafe.Pointer, namelen int32) (err error) {
	r1, _, e1 := syscall.Syscall(procbind.Addr(), 3, uintptr(s), uintptr(name), uintptr(namelen))
	if r1 == socket_error {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func cancelIoEx(file syscall.Handle, o *syscall.Overlapped) (err error) {
	r1, _, e1 := syscall.Syscall(procCancelIoEx.Addr(), 2, uintptr(file), uintptr(unsafe.Pointer(o)), 0)
	if r1 == 0 {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func createIoCompletionPort(file syscall.Handle, port syscall.Handle, key uintptr, threadCount uint32) (newport syscall.Handle, err error) {
	r0, _, e1 := syscall.Syscall6(procCreateIoCompletionPort.Addr(), 4, uintptr(file), uintptr(port), uintptr(key), uintptr(threadCount), 0, 0)
	newport = syscall.Handle(r0)
	if newport == 0 {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func getQueuedCompletionStatus(port syscall.Handle, bytes *uint32, key *uintptr, o **ioOperation, timeout uint32) (err error) {
	r1, _, e1 := syscall.Syscall6(procGetQueuedCompletionStatus.Addr(), 5, uintptr(port), uintptr(unsafe.Pointer(bytes)), uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(o)), uintptr(timeout), 0)
	if r1 == 0 {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func setFileCompletionNotificationModes(h syscall.Handle, flags uint8) (err error) {
	r1, _, e1 := syscall.Syscall(procSetFileCompletionNotificationModes.Addr(), 2, uintptr(h), uintptr(flags), 0)
	if r1 == 0 {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func timeBeginPeriod(period uint32) (n int32) {
	r0, _, _ := syscall.Syscall(proctimeBeginPeriod.Addr(), 1, uintptr(period), 0, 0)
	n = int32(r0)
	return
}
