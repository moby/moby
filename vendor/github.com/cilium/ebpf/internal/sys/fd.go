package sys

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/cilium/ebpf/internal/testutils/fdtrace"
	"github.com/cilium/ebpf/internal/unix"
)

var ErrClosedFd = unix.EBADF

type FD struct {
	raw int
}

func newFD(value int) *FD {
	fdtrace.TraceFD(value, 1)

	fd := &FD{value}
	runtime.SetFinalizer(fd, (*FD).finalize)
	return fd
}

// finalize is set as the FD's runtime finalizer and
// sends a leak trace before calling FD.Close().
func (fd *FD) finalize() {
	if fd.raw < 0 {
		return
	}

	fdtrace.LeakFD(fd.raw)

	_ = fd.Close()
}

// NewFD wraps a raw fd with a finalizer.
//
// You must not use the raw fd after calling this function, since the underlying
// file descriptor number may change. This is because the BPF UAPI assumes that
// zero is not a valid fd value.
func NewFD(value int) (*FD, error) {
	if value < 0 {
		return nil, fmt.Errorf("invalid fd %d", value)
	}

	fd := newFD(value)
	if value != 0 {
		return fd, nil
	}

	dup, err := fd.Dup()
	_ = fd.Close()
	return dup, err
}

func (fd *FD) String() string {
	return strconv.FormatInt(int64(fd.raw), 10)
}

func (fd *FD) Int() int {
	return fd.raw
}

func (fd *FD) Uint() uint32 {
	if fd.raw < 0 || int64(fd.raw) > math.MaxUint32 {
		// Best effort: this is the number most likely to be an invalid file
		// descriptor. It is equal to -1 (on two's complement arches).
		return math.MaxUint32
	}
	return uint32(fd.raw)
}

func (fd *FD) Close() error {
	if fd.raw < 0 {
		return nil
	}

	return unix.Close(fd.Disown())
}

// Disown destroys the FD and returns its raw file descriptor without closing
// it. After this call, the underlying fd is no longer tied to the FD's
// lifecycle.
func (fd *FD) Disown() int {
	value := fd.raw
	fdtrace.ForgetFD(value)
	fd.raw = -1

	runtime.SetFinalizer(fd, nil)
	return value
}

func (fd *FD) Dup() (*FD, error) {
	if fd.raw < 0 {
		return nil, ErrClosedFd
	}

	// Always require the fd to be larger than zero: the BPF API treats the value
	// as "no argument provided".
	dup, err := unix.FcntlInt(uintptr(fd.raw), unix.F_DUPFD_CLOEXEC, 1)
	if err != nil {
		return nil, fmt.Errorf("can't dup fd: %v", err)
	}

	return newFD(dup), nil
}

// File takes ownership of FD and turns it into an [*os.File].
//
// You must not use the FD after the call returns.
//
// Returns nil if the FD is not valid.
func (fd *FD) File(name string) *os.File {
	if fd.raw < 0 {
		return nil
	}

	return os.NewFile(uintptr(fd.Disown()), name)
}

// ObjGetTyped wraps [ObjGet] with a readlink call to extract the type of the
// underlying bpf object.
func ObjGetTyped(attr *ObjGetAttr) (*FD, ObjType, error) {
	fd, err := ObjGet(attr)
	if err != nil {
		return nil, 0, err
	}

	typ, err := readType(fd)
	if err != nil {
		_ = fd.Close()
		return nil, 0, fmt.Errorf("reading fd type: %w", err)
	}

	return fd, typ, nil
}

// readType returns the bpf object type of the file descriptor by calling
// readlink(3). Returns an error if the file descriptor does not represent a bpf
// object.
func readType(fd *FD) (ObjType, error) {
	s, err := os.Readlink(filepath.Join("/proc/self/fd/", fd.String()))
	if err != nil {
		return 0, fmt.Errorf("readlink fd %d: %w", fd.Int(), err)
	}

	s = strings.TrimPrefix(s, "anon_inode:")

	switch s {
	case "bpf-map":
		return BPF_TYPE_MAP, nil
	case "bpf-prog":
		return BPF_TYPE_PROG, nil
	case "bpf-link":
		return BPF_TYPE_LINK, nil
	}

	return 0, fmt.Errorf("unknown type %s of fd %d", s, fd.Int())
}
