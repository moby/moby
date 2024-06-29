package graphdriver // import "github.com/docker/docker/daemon/graphdriver"

import (
	"github.com/docker/docker/daemon/internal/fstype"
	"github.com/moby/sys/mountinfo"
)

// List of drivers that should be used in an order
var priority = "overlay2,fuse-overlayfs,btrfs,zfs,vfs"

// NewFsChecker returns a checker configured for the provided FsMagic
func NewFsChecker(t fstype.FsMagic) Checker {
	return &fsChecker{
		t: t,
	}
}

type fsChecker struct {
	t fstype.FsMagic
}

func (c *fsChecker) IsMounted(path string) bool {
	fsType, _ := fstype.GetFSMagic(path)
	return fsType == c.t
}

// NewDefaultChecker returns a check that parses /proc/mountinfo to check
// if the specified path is mounted.
func NewDefaultChecker() Checker {
	return &defaultChecker{}
}

type defaultChecker struct{}

func (c *defaultChecker) IsMounted(path string) bool {
	m, _ := mountinfo.Mounted(path)
	return m
}
