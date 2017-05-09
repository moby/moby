package local

import "golang.org/x/sys/unix"

func unmount(path string) error {
	return unix.Unmount(path, unix.MNT_DETACH)
}
