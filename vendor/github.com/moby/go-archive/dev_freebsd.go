//go:build freebsd

package archive

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func mknod(path string, mode uint32, dev uint64) error {
	return unix.Mknod(path, mode, dev)
}

func mknodInRoot(root *os.Root, path string, mode uint32, dev uint64) error {
	parent, err := root.OpenFile(filepath.Dir(path), os.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer parent.Close()

	return unix.Mknodat(int(parent.Fd()), filepath.Base(path), mode, dev)
}
