package fuse

import (
	"syscall"
)

const PAGESIZE = 4096

const (
	_DEFAULT_BACKGROUND_TASKS = 12
)

// Status is the errno number that a FUSE call returns to the kernel.
type Status int32

const (
	OK      = Status(0)
	EACCES  = Status(syscall.EACCES)
	EBUSY   = Status(syscall.EBUSY)
	EINVAL  = Status(syscall.EINVAL)
	EIO     = Status(syscall.EIO)
	ENOENT  = Status(syscall.ENOENT)
	ENOSYS  = Status(syscall.ENOSYS)
	ENODATA = Status(syscall.ENODATA)
	ENOTDIR = Status(syscall.ENOTDIR)
	EPERM   = Status(syscall.EPERM)
	ERANGE  = Status(syscall.ERANGE)
	EXDEV   = Status(syscall.EXDEV)
	EBADF   = Status(syscall.EBADF)
	ENODEV  = Status(syscall.ENODEV)
	EROFS   = Status(syscall.EROFS)
)

type ForgetIn struct {
	InHeader

	Nlookup uint64
}

// batch forget is handled internally.
type _ForgetOne struct {
	NodeId  uint64
	Nlookup uint64
}

// batch forget is handled internally.
type _BatchForgetIn struct {
	InHeader
	Count uint32
	Dummy uint32
}

type MkdirIn struct {
	InHeader
	Mode  uint32
	Umask uint32
}

type RenameIn struct {
	InHeader
	Newdir uint64
}

type Rename2In struct {
	InHeader
	Newdir  uint64
	Flags   uint32
	Padding uint32
}

type LinkIn struct {
	InHeader
	Oldnodeid uint64
}

type Owner struct {
	Uid uint32
	Gid uint32
}

const ( // SetAttrIn.Valid
	FATTR_MODE      = (1 << 0)
	FATTR_UID       = (1 << 1)
	FATTR_GID       = (1 << 2)
	FATTR_SIZE      = (1 << 3)
	FATTR_ATIME     = (1 << 4)
	FATTR_MTIME     = (1 << 5)
	FATTR_FH        = (1 << 6)
	FATTR_ATIME_NOW = (1 << 7)
	FATTR_MTIME_NOW = (1 << 8)
	FATTR_LOCKOWNER = (1 << 9)
	FATTR_CTIME     = (1 << 10)
)

type SetAttrInCommon struct {
	InHeader

	Valid     uint32
	Padding   uint32
	Fh        uint64
	Size      uint64
	LockOwner uint64
	Atime     uint64
	Mtime     uint64
	Ctime     uint64
	Atimensec uint32
	Mtimensec uint32
	Ctimensec uint32
	Mode      uint32
	Unused4   uint32
	Owner
	Unused5 uint32
}

const RELEASE_FLUSH = (1 << 0)

type ReleaseIn struct {
	InHeader
	Fh           uint64
	Flags        uint32
	ReleaseFlags uint32
	LockOwner    uint64
}

type OpenIn struct {
	InHeader
	Flags uint32
	Mode  uint32
}

const (
	// OpenOut.Flags
	FOPEN_DIRECT_IO   = (1 << 0)
	FOPEN_KEEP_CACHE  = (1 << 1)
	FOPEN_NONSEEKABLE = (1 << 2)
)

type OpenOut struct {
	Fh        uint64
	OpenFlags uint32
	Padding   uint32
}

// To be set in InitIn/InitOut.Flags.
const (
	CAP_ASYNC_READ       = (1 << 0)
	CAP_POSIX_LOCKS      = (1 << 1)
	CAP_FILE_OPS         = (1 << 2)
	CAP_ATOMIC_O_TRUNC   = (1 << 3)
	CAP_EXPORT_SUPPORT   = (1 << 4)
	CAP_BIG_WRITES       = (1 << 5)
	CAP_DONT_MASK        = (1 << 6)
	CAP_SPLICE_WRITE     = (1 << 7)
	CAP_SPLICE_MOVE      = (1 << 8)
	CAP_SPLICE_READ      = (1 << 9)
	CAP_FLOCK_LOCKS      = (1 << 10)
	CAP_IOCTL_DIR        = (1 << 11)
	CAP_AUTO_INVAL_DATA  = (1 << 12)
	CAP_READDIRPLUS      = (1 << 13)
	CAP_READDIRPLUS_AUTO = (1 << 14)
	CAP_ASYNC_DIO        = (1 << 15)
	CAP_WRITEBACK_CACHE  = (1 << 16)
	CAP_NO_OPEN_SUPPORT  = (1 << 17)
)

type InitIn struct {
	InHeader

	Major        uint32
	Minor        uint32
	MaxReadAhead uint32
	Flags        uint32
}

type InitOut struct {
	Major               uint32
	Minor               uint32
	MaxReadAhead        uint32
	Flags               uint32
	MaxBackground       uint16
	CongestionThreshold uint16
	MaxWrite            uint32
	TimeGran            uint32
	Unused              [9]uint32
}

type _CuseInitIn struct {
	InHeader
	Major  uint32
	Minor  uint32
	Unused uint32
	Flags  uint32
}

type _CuseInitOut struct {
	Major    uint32
	Minor    uint32
	Unused   uint32
	Flags    uint32
	MaxRead  uint32
	MaxWrite uint32
	DevMajor uint32
	DevMinor uint32
	Spare    [10]uint32
}

type InterruptIn struct {
	InHeader
	Unique uint64
}

type _BmapIn struct {
	InHeader
	Block     uint64
	Blocksize uint32
	Padding   uint32
}

type _BmapOut struct {
	Block uint64
}

const (
	FUSE_IOCTL_COMPAT       = (1 << 0)
	FUSE_IOCTL_UNRESTRICTED = (1 << 1)
	FUSE_IOCTL_RETRY        = (1 << 2)
)

type _IoctlIn struct {
	InHeader
	Fh      uint64
	Flags   uint32
	Cmd     uint32
	Arg     uint64
	InSize  uint32
	OutSize uint32
}

type _IoctlOut struct {
	Result  int32
	Flags   uint32
	InIovs  uint32
	OutIovs uint32
}

type _PollIn struct {
	InHeader
	Fh      uint64
	Kh      uint64
	Flags   uint32
	Padding uint32
}

type _PollOut struct {
	Revents uint32
	Padding uint32
}

type _NotifyPollWakeupOut struct {
	Kh uint64
}

type WriteOut struct {
	Size    uint32
	Padding uint32
}

type GetXAttrOut struct {
	Size    uint32
	Padding uint32
}

type _FileLock struct {
	Start uint64
	End   uint64
	Typ   uint32
	Pid   uint32
}

type _LkIn struct {
	InHeader
	Fh      uint64
	Owner   uint64
	Lk      _FileLock
	LkFlags uint32
	Padding uint32
}

type _LkOut struct {
	Lk _FileLock
}

// For AccessIn.Mask.
const (
	X_OK = 1
	W_OK = 2
	R_OK = 4
	F_OK = 0
)

type AccessIn struct {
	InHeader
	Mask    uint32
	Padding uint32
}

type FsyncIn struct {
	InHeader
	Fh         uint64
	FsyncFlags uint32
	Padding    uint32
}

type OutHeader struct {
	Length uint32
	Status int32
	Unique uint64
}

type NotifyInvalInodeOut struct {
	Ino    uint64
	Off    int64
	Length int64
}

type NotifyInvalEntryOut struct {
	Parent  uint64
	NameLen uint32
	Padding uint32
}

type NotifyInvalDeleteOut struct {
	Parent  uint64
	Child   uint64
	NameLen uint32
	Padding uint32
}

const (
	NOTIFY_POLL         = -1
	NOTIFY_INVAL_INODE  = -2
	NOTIFY_INVAL_ENTRY  = -3
	NOTIFY_STORE        = -4
	NOTIFY_RETRIEVE     = -5
	NOTIFY_INVAL_DELETE = -6
	NOTIFY_CODE_MAX     = -6
)

type FlushIn struct {
	InHeader
	Fh        uint64
	Unused    uint32
	Padding   uint32
	LockOwner uint64
}

type EntryOut struct {
	NodeId         uint64
	Generation     uint64
	EntryValid     uint64
	AttrValid      uint64
	EntryValidNsec uint32
	AttrValidNsec  uint32
	Attr
}

type AttrOut struct {
	AttrValid     uint64
	AttrValidNsec uint32
	Dummy         uint32
	Attr
}

type CreateOut struct {
	EntryOut
	OpenOut
}

type Context struct {
	Owner
	Pid uint32
}

type InHeader struct {
	Length uint32
	Opcode int32
	Unique uint64
	NodeId uint64
	Context
	Padding uint32
}

type StatfsOut struct {
	Blocks  uint64
	Bfree   uint64
	Bavail  uint64
	Files   uint64
	Ffree   uint64
	Bsize   uint32
	NameLen uint32
	Frsize  uint32
	Padding uint32
	Spare   [6]uint32
}

// _Dirent is what we send to the kernel, but we offer DirEntry and
// DirEntryList to the user.
type _Dirent struct {
	Ino     uint64
	Off     uint64
	NameLen uint32
	Typ     uint32
}

const (
	READ_LOCKOWNER = (1 << 1)
)

const (
	WRITE_CACHE     = (1 << 0)
	WRITE_LOCKOWNER = (1 << 1)
)

type FallocateIn struct {
	InHeader
	Fh      uint64
	Offset  uint64
	Length  uint64
	Mode    uint32
	Padding uint32
}
