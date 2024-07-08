//go:build !windows

package snapshotter

import (
	"github.com/containerd/containerd/mount"
	"github.com/moby/sys/mountinfo"
	"golang.org/x/sys/unix"
)

// isMounted parses /proc/mountinfo to check whether the specified path
// is mounted.
func isMounted(path string) bool {
	m, _ := mountinfo.Mounted(path)
	return m
}

func unmount(target string) error {
	return mount.Unmount(target, unix.MNT_DETACH)
}
