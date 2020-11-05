package mount // import "github.com/docker/docker/pkg/mount"

import (
	sysmount "github.com/moby/sys/mount"
)

// Deprecated: use github.com/moby/sys/mount instead.
//nolint:golint
var (
	ForceMount        = sysmount.Mount // a deprecated synonym
	MakeMount         = sysmount.MakeMount
	MakePrivate       = sysmount.MakePrivate
	MakeRPrivate      = sysmount.MakeRPrivate
	MakeRShared       = sysmount.MakeRShared
	MakeRSlave        = sysmount.MakeRSlave
	MakeRUnbindable   = sysmount.MakeRUnbindable
	MakeShared        = sysmount.MakeShared
	MakeSlave         = sysmount.MakeSlave
	MakeUnbindable    = sysmount.MakeUnbindable
	MergeTmpfsOptions = sysmount.MergeTmpfsOptions
	Mount             = sysmount.Mount
	RecursiveUnmount  = sysmount.RecursiveUnmount
	Unmount           = sysmount.Unmount
)

// Deprecated: use github.com/moby/sys/mount instead.
//nolint:golint
const (
	RDONLY      = sysmount.RDONLY
	NOSUID      = sysmount.NOSUID
	NOEXEC      = sysmount.NOEXEC
	SYNCHRONOUS = sysmount.SYNCHRONOUS
	NOATIME     = sysmount.NOATIME
	BIND        = sysmount.BIND
	DIRSYNC     = sysmount.DIRSYNC
	MANDLOCK    = sysmount.MANDLOCK
	NODEV       = sysmount.NODEV
	NODIRATIME  = sysmount.NODIRATIME
	UNBINDABLE  = sysmount.UNBINDABLE
	RUNBINDABLE = sysmount.RUNBINDABLE
	PRIVATE     = sysmount.PRIVATE
	RPRIVATE    = sysmount.RPRIVATE
	SHARED      = sysmount.SHARED
	RSHARED     = sysmount.RSHARED
	SLAVE       = sysmount.SLAVE
	RSLAVE      = sysmount.RSLAVE
	RBIND       = sysmount.RBIND
	RELATIME    = sysmount.RELATIME
	REMOUNT     = sysmount.REMOUNT
	STRICTATIME = sysmount.STRICTATIME
)
