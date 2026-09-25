//go:build darwin

package archive

import (
	"os"

	"golang.org/x/sys/unix"
)

func mknod(path string, mode uint32, dev uint64) error {
	return unix.Mknod(path, mode, int(dev)) // #nosec G115 -- Required conversion for the platform-specific Mknod API.
}

func mknodInRoot(root *os.Root, path string, mode uint32, dev uint64) error {
	abs, err := fsRootPath(root.Name(), path)
	if err != nil {
		return err
	}
	return unix.Mknod(abs, mode, int(dev)) // #nosec G115 -- Required conversion for the platform-specific Mknod API.
}
