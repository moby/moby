package fuse

import (
	"os"
	"syscall"
)

const (
	FUSE_ROOT_ID = 1

	FUSE_UNKNOWN_INO = 0xffffffff

	CUSE_UNRESTRICTED_IOCTL = (1 << 0)

	FUSE_LK_FLOCK = (1 << 0)

	FUSE_IOCTL_MAX_IOV = 256

	FUSE_POLL_SCHEDULE_NOTIFY = (1 << 0)

	CUSE_INIT_INFO_MAX = 4096

	S_IFDIR = syscall.S_IFDIR
	S_IFREG = syscall.S_IFREG
	S_IFLNK = syscall.S_IFLNK
	S_IFIFO = syscall.S_IFIFO

	CUSE_INIT = 4096

	O_ANYWRITE = uint32(os.O_WRONLY | os.O_RDWR | os.O_APPEND | os.O_CREATE | os.O_TRUNC)
)
