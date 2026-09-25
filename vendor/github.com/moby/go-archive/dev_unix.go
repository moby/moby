//go:build !darwin && !freebsd && !windows

package archive

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func mknod(path string, mode uint32, dev uint64) error {
	return unix.Mknod(path, mode, int(dev)) // #nosec G115 -- Required conversion for the platform-specific Mknod API.
}

func mknodInRoot(root *os.Root, path string, mode uint32, dev uint64) error {
	parent, err := root.OpenFile(filepath.Dir(path), os.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer parent.Close()

	return unix.Mknodat(int(parent.Fd()), filepath.Base(path), mode, int(dev)) // #nosec G115 -- Required conversion for the platform-specific Mknod API.
}
