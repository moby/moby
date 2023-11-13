//go:build !windows

package snapshotter

import (
	"github.com/containerd/containerd/mount"
	"golang.org/x/sys/unix"

	"github.com/docker/docker/daemon/graphdriver"
)

func checker() graphdriver.Checker {
	return graphdriver.NewDefaultChecker()
}

func unmount(target string) error {
	return mount.Unmount(target, unix.MNT_DETACH)
}
