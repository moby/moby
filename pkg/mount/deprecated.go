package mount // import "github.com/docker/docker/pkg/mount"

// Deprecated: this package is not maintained and will be removed.
// Use github.com/moby/sys/mount and github.com/moby/sys/mountinfo instead.

import (
	sysmount "github.com/moby/sys/mount"
	"github.com/moby/sys/mountinfo"
)

// Deprecated: use github.com/moby/sys/mount instead.
//nolint:golint
var (
	Mount            = sysmount.Mount
	ForceMount       = sysmount.Mount // a deprecated synonym
	Unmount          = sysmount.Unmount
	RecursiveUnmount = sysmount.RecursiveUnmount
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

// Deprecated: use github.com/moby/sys/mount instead.
//nolint:golint
var (
	MergeTmpfsOptions = sysmount.MergeTmpfsOptions
)

//nolint:golint
type (
	// FilterFunc is a type.
	// Deprecated: use github.com/moby/sys/mountinfo instead.
	FilterFunc = func(*Info) (skip, stop bool)

	// Info is a type
	// Deprecated: use github.com/moby/sys/mountinfo instead.
	Info struct {
		ID, Parent, Major, Minor                                  int
		Root, Mountpoint, Opts, Optional, Fstype, Source, VfsOpts string
	}
)

// Deprecated: use github.com/moby/sys/mountinfo instead.
//nolint:golint
var (
	Mounted           = mountinfo.Mounted
	PrefixFilter      = mountinfo.PrefixFilter
	SingleEntryFilter = mountinfo.SingleEntryFilter
	ParentsFilter     = mountinfo.ParentsFilter
	FstypeFilter      = mountinfo.FSTypeFilter
)

// GetMounts is a function.
//
// Deprecated: use github.com/moby/sys/mountinfo.GetMounts() instead.
//nolint:golint
func GetMounts(f FilterFunc) ([]*Info, error) {
	fi := func(i *mountinfo.Info) (skip, stop bool) {
		return f(toLegacyInfo(i))
	}

	mounts, err := mountinfo.GetMounts(fi)
	if err != nil {
		return nil, err
	}
	mi := make([]*Info, len(mounts))
	for i, m := range mounts {
		mi[i] = toLegacyInfo(m)
	}
	return mi, nil
}

func toLegacyInfo(m *mountinfo.Info) *Info {
	return &Info{
		ID:         m.ID,
		Parent:     m.Parent,
		Major:      m.Major,
		Minor:      m.Minor,
		Root:       m.Root,
		Mountpoint: m.Mountpoint,
		Opts:       m.Options,
		Fstype:     m.FSType,
		Source:     m.Source,
		VfsOpts:    m.VFSOptions,
	}
}
