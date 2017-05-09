// +build !linux,!windows

package local

import "golang.org/x/sys/unix"

func unmount(path string) error {
	return unix.Unmount(path, 0)
}
