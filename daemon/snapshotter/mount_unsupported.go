//go:build !linux && !windows

package snapshotter

import "github.com/containerd/containerd/v2/core/mount"

func unmount(target string) error {
	return mount.Unmount(target, 0)
}
