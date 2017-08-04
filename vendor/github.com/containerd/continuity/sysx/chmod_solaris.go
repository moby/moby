package sysx

import "golang.org/x/sys/unix"

const (
	AtSymlinkNofollow = unix.AT_SYMLINK_NOFOLLOW
)

func Fchmodat(dirfd int, path string, mode uint32, flags int) error {
	return unix.Fchmodat(dirfd, path, mode, flags)
}
