// +build !windows,!freebsd

package fsutil

import (
	"syscall"

	"github.com/tonistiigi/fsutil/types"
)

func createSpecialFile(path string, mode uint32, stat *types.Stat) error {
	return syscall.Mknod(path, mode, int(mkdev(stat.Devmajor, stat.Devminor)))
}
