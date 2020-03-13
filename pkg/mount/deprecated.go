package mount // import "github.com/docker/docker/pkg/mount"

import (
	sysmount "github.com/moby/sys/mount"
	"github.com/moby/sys/mountinfo"
)

//nolint:golint
var (
	Mount            = sysmount.Mount
	ForceMount       = sysmount.Mount // Deprecated: use Mount instead.
	Unmount          = sysmount.Unmount
	RecursiveUnmount = sysmount.RecursiveUnmount
)

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

//nolint:golint
var MergeTmpfsOptions = sysmount.MergeTmpfsOptions

//nolint:golint
type (
	FilterFunc = mountinfo.FilterFunc
	Info       = mountinfo.Info
)

//nolint:golint
var (
	Mounted   = mountinfo.Mounted
	GetMounts = mountinfo.GetMounts

	PrefixFilter      = mountinfo.PrefixFilter
	SingleEntryFilter = mountinfo.SingleEntryFilter
	ParentsFilter     = mountinfo.ParentsFilter
	FstypeFilter      = mountinfo.FstypeFilter
)
