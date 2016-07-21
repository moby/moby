// Random odds and ends.

package fuse

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"syscall"
	"unsafe"
)

func (code Status) String() string {
	if code <= 0 {
		return []string{
			"OK",
			"NOTIFY_POLL",
			"NOTIFY_INVAL_INODE",
			"NOTIFY_INVAL_ENTRY",
			"NOTIFY_INVAL_STORE",
			"NOTIFY_INVAL_RETRIEVE",
			"NOTIFY_INVAL_DELETE",
		}[-code]
	}
	return fmt.Sprintf("%d=%v", int(code), syscall.Errno(code))
}

func (code Status) Ok() bool {
	return code == OK
}

// ToStatus extracts an errno number from Go error objects.  If it
// fails, it logs an error and returns ENOSYS.
func ToStatus(err error) Status {
	switch err {
	case nil:
		return OK
	case os.ErrPermission:
		return EPERM
	case os.ErrExist:
		return Status(syscall.EEXIST)
	case os.ErrNotExist:
		return ENOENT
	case os.ErrInvalid:
		return EINVAL
	}

	switch t := err.(type) {
	case syscall.Errno:
		return Status(t)
	case *os.SyscallError:
		return Status(t.Err.(syscall.Errno))
	case *os.PathError:
		return ToStatus(t.Err)
	case *os.LinkError:
		return ToStatus(t.Err)
	}
	log.Println("can't convert error type:", err)
	return ENOSYS
}

func toSlice(dest *[]byte, ptr unsafe.Pointer, byteCount uintptr) {
	h := (*reflect.SliceHeader)(unsafe.Pointer(dest))
	*h = reflect.SliceHeader{
		Data: uintptr(ptr),
		Len:  int(byteCount),
		Cap:  int(byteCount),
	}
}

func CurrentOwner() *Owner {
	return &Owner{
		Uid: uint32(os.Getuid()),
		Gid: uint32(os.Getgid()),
	}
}

func init() {
	p := syscall.Getpagesize()
	if p != PAGESIZE {
		log.Panicf("page size incorrect: %d", p)
	}
}
