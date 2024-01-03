//go:build freebsd || openbsd
// +build freebsd openbsd

package mount

import "golang.org/x/sys/unix"

const (
	// RDONLY will mount the filesystem as read-only.
	RDONLY = unix.MNT_RDONLY

	// NOSUID will not allow set-user-identifier or set-group-identifier bits to
	// take effect.
	NOSUID = unix.MNT_NOSUID

	// NOEXEC will not allow execution of any binaries on the mounted file system.
	NOEXEC = unix.MNT_NOEXEC

	// SYNCHRONOUS will allow any I/O to the file system to be done synchronously.
	SYNCHRONOUS = unix.MNT_SYNCHRONOUS

	// NOATIME will not update the file access time when reading from a file.
	NOATIME = unix.MNT_NOATIME
)

// These flags are unsupported.
const (
	BIND        = 0
	DIRSYNC     = 0
	MANDLOCK    = 0
	NODEV       = 0
	NODIRATIME  = 0
	UNBINDABLE  = 0
	RUNBINDABLE = 0
	PRIVATE     = 0
	RPRIVATE    = 0
	SHARED      = 0
	RSHARED     = 0
	SLAVE       = 0
	RSLAVE      = 0
	RBIND       = 0
	RELATIME    = 0
	REMOUNT     = 0
	STRICTATIME = 0
	mntDetach   = 0
)
