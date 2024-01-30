package snapshotter

import "github.com/containerd/containerd/v2/core/mount"

type winChecker struct{}

func (c *winChecker) IsMounted(path string) bool {
	return false
}

func checker() *winChecker {
	return &winChecker{}
}

func unmount(target string) error {
	return mount.Unmount(target, 0)
}
