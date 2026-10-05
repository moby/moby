package sys

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/efw"
	"github.com/cilium/ebpf/internal/unix"
)

// BPF calls the BPF syscall wrapper in ebpfapi.dll.
//
// Any pointers contained in attr must use the Pointer type from this package.
//
// The implementation lives in https://github.com/microsoft/ebpf-for-windows/blob/main/libs/api/bpf_syscall.cpp
func BPF(cmd Cmd, attr unsafe.Pointer, size uintptr) (uintptr, error) {
	// On Linux we need to guard against preemption by the profiler here. On
	// Windows it seems like a cgocall may not be preempted:
	// https://github.com/golang/go/blob/8b51146c698bcfcc2c2b73fa9390db5230f2ce0a/src/runtime/os_windows.go#L1240-L1246

	addr, err := efw.BPF.Find()
	if err != nil {
		return 0, err
	}

	// Using [LazyProc.Call] forces attr to escape, which isn't the case when using syscall.Syscall directly.
	r1, _, lastError := syscall.SyscallN(addr, uintptr(cmd), uintptr(attr), size)

	if ret := int(efw.Int(r1)); ret < 0 {
		errNo := unix.Errno(-ret)
		if errNo == unix.EINVAL && lastError == windows.ERROR_CALL_NOT_IMPLEMENTED {
			return 0, internal.ErrNotSupportedOnOS
		}
		return 0, wrappedErrno{errNo}
	}

	return r1, nil
}

// ObjGetTyped retrieves an pinned object and its type.
func ObjGetTyped(attr *ObjGetAttr) (*FD, ObjType, error) {
	fd, err := ObjGet(attr)
	if err != nil {
		return nil, 0, err
	}

	efwType, err := efw.EbpfObjectGetInfoByFd(fd.Int(), nil, nil)
	if err != nil {
		_ = fd.Close()
		return nil, 0, err
	}

	switch efwType {
	case efw.EBPF_OBJECT_UNKNOWN:
		return fd, BPF_TYPE_UNSPEC, nil
	case efw.EBPF_OBJECT_MAP:
		return fd, BPF_TYPE_MAP, nil
	case efw.EBPF_OBJECT_LINK:
		return fd, BPF_TYPE_LINK, nil
	case efw.EBPF_OBJECT_PROGRAM:
		return fd, BPF_TYPE_PROG, nil
	default:
		return nil, 0, fmt.Errorf("unrecognized object type %v", efwType)
	}
}
