package snapshotter

import "github.com/containerd/containerd/mount"

func isMounted(string) bool { return false }

func unmount(target string) error {
	return mount.Unmount(target, 0)
}
