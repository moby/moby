// +build freebsd

package fsutil

import (
	"github.com/tonistiigi/fsutil/types"
	"golang.org/x/sys/unix"
)

func createSpecialFile(path string, mode uint32, stat *types.Stat) error {
	dev := unix.Mkdev(uint32(stat.Devmajor), uint32(stat.Devminor))

	return unix.Mknod(path, mode, dev)
}
