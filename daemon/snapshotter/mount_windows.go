package snapshotter

import "github.com/containerd/containerd/v2/core/mount"

func isMounted(string) bool { return false }

func unmount(target string) error {
	return mount.Unmount(target, 0)
}
