package sys

import (
	"runtime"
	"unsafe"

	"github.com/cilium/ebpf/internal/unix"
)

// ENOTSUPP is a Linux internal error code that has leaked into UAPI.
//
// It is not the same as ENOTSUP or EOPNOTSUPP.
const ENOTSUPP = unix.Errno(524)

// Info is implemented by all structs that can be passed to the ObjInfo syscall.
//
//	MapInfo
//	ProgInfo
//	LinkInfo
//	BtfInfo
//	TokenInfo
type Info interface {
	info() (unsafe.Pointer, uint32)
}

var _ Info = (*MapInfo)(nil)

func (i *MapInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

var _ Info = (*ProgInfo)(nil)

func (i *ProgInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

var _ Info = (*LinkInfo)(nil)

func (i *LinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *TracingLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *CgroupLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *NetNsLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *XDPLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *TcxLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *NetfilterLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *NetkitLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *KprobeMultiLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *UprobeMultiLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *RawTracepointLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *KprobeLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *UprobeLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *TracepointLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *EventLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

var _ Info = (*BtfInfo)(nil)

func (i *BtfInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

func (i *PerfEventLinkInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

var _ Info = (*TokenInfo)(nil)

func (i *TokenInfo) info() (unsafe.Pointer, uint32) {
	return unsafe.Pointer(i), uint32(unsafe.Sizeof(*i))
}

// ObjInfo retrieves information about a BPF Fd.
//
// info may be one of MapInfo, ProgInfo, LinkInfo, BtfInfo and TokenInfo.
func ObjInfo(fd *FD, info Info) error {
	ptr, len := info.info()
	err := ObjGetInfoByFd(&ObjGetInfoByFdAttr{
		BpfFd:   fd.Uint(),
		InfoLen: len,
		Info:    UnsafePointer(ptr),
	})
	runtime.KeepAlive(fd)
	return err
}

// BPFObjName is a null-terminated string made up of
// 'A-Za-z0-9_' characters.
type ObjName [BPF_OBJ_NAME_LEN]byte

// NewObjName truncates the result if it is too long.
func NewObjName(name string) ObjName {
	var result ObjName
	copy(result[:BPF_OBJ_NAME_LEN-1], name)
	return result
}

// LogLevel controls the verbosity of the kernel's eBPF program verifier.
type LogLevel uint32

const (
	BPF_LOG_LEVEL1 LogLevel = 1 << iota
	BPF_LOG_LEVEL2
	BPF_LOG_STATS
)

// MapID uniquely identifies a bpf_map.
type MapID uint32

// ProgramID uniquely identifies a bpf_map.
type ProgramID uint32

// LinkID uniquely identifies a bpf_link.
type LinkID uint32

// BTFID uniquely identifies a BTF blob loaded into the kernel.
type BTFID uint32

// TypeID identifies a type in a BTF blob.
type TypeID uint32

// Flags used by bpf_mprog.
const (
	BPF_F_REPLACE = 1 << (iota + 2)
	BPF_F_BEFORE
	BPF_F_AFTER
	BPF_F_ID
	BPF_F_LINK_MPROG = 1 << 13 // aka BPF_F_LINK
)

// Flags used by BPF_PROG_LOAD.
const (
	BPF_F_SLEEPABLE          = 1 << 4
	BPF_F_XDP_HAS_FRAGS      = 1 << 5
	BPF_F_XDP_DEV_BOUND_ONLY = 1 << 6
)

const BPF_TAG_SIZE = 8
const BPF_OBJ_NAME_LEN = 16

// wrappedErrno wraps [unix.Errno] to prevent direct comparisons with
// syscall.E* or unix.E* constants.
//
// You should never export an error of this type.
type wrappedErrno struct {
	unix.Errno
}

func (we wrappedErrno) Unwrap() error {
	return we.Errno
}

func (we wrappedErrno) Error() string {
	if we.Errno == ENOTSUPP {
		return "operation not supported"
	}
	return we.Errno.Error()
}

type syscallError struct {
	error
	errno unix.Errno
}

func Error(err error, errno unix.Errno) error {
	return &syscallError{err, errno}
}

func (se *syscallError) Is(target error) bool {
	return target == se.error
}

func (se *syscallError) Unwrap() error {
	return se.errno
}
