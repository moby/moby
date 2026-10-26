package sys

import (
	"math"
	"runtime"
	"strconv"

	"github.com/cilium/ebpf/internal/testutils/testmain"
	"github.com/cilium/ebpf/internal/unix"
)

var ErrClosedFd = unix.EBADF

// A value for an invalid fd.
//
// Luckily this is consistent across Linux and Windows.
//
// See https://github.com/microsoft/ebpf-for-windows/blob/54632eb360c560ebef2f173be1a4a4625d540744/include/ebpf_api.h#L25
const invalidFd = -1

func newFD(value int) *FD {
	testmain.TraceFD(value, 1)

	fd := &FD{raw: value}
	fd.cleanup = runtime.AddCleanup(fd, func(raw int) {
		testmain.LeakFD(raw)
		_ = unix.Close(raw)
	}, fd.raw)
	return fd
}

func (fd *FD) String() string {
	return strconv.FormatInt(int64(fd.raw), 10)
}

func (fd *FD) Int() int {
	return int(fd.raw)
}

func (fd *FD) Uint() uint32 {
	if fd.raw == invalidFd {
		// Best effort: this is the number most likely to be an invalid file
		// descriptor. It is equal to -1 (on two's complement arches).
		return math.MaxUint32
	}
	return uint32(fd.raw)
}

// Disown destroys the FD and returns its raw file descriptor without closing
// it. After this call, the underlying fd is no longer tied to the FD's
// lifecycle.
func (fd *FD) Disown() int {
	value := fd.raw
	testmain.ForgetFD(value)
	fd.raw = invalidFd

	fd.cleanup.Stop()
	return value
}
