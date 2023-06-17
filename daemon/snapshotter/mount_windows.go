package snapshotter

import "github.com/containerd/containerd/mount"

type winChecker struct {
}

func (c *winChecker) IsMounted(path string) bool {
	return false
}

func checker() *winChecker {
	return &winChecker{}
}

func unmount(target string) error {
	return mount.Unmount(target, 0)
}
