package snapshotter

import (
	"github.com/containerd/containerd/mount"
	"golang.org/x/sys/unix"
)

type darwinChecker struct{}

func (c *darwinChecker) IsMounted(path string) bool {
	return false
}

func checker() *darwinChecker {
	return &darwinChecker{}
}

func unmount(target string) error {
	return mount.Unmount(target, unix.MNT_FORCE)
}
