//go:build linux || freebsd || netbsd || openbsd || dragonfly

package fsutil

import (
	"os"
	"syscall"

	"github.com/tonistiigi/fsutil/types"
)

func handleRootTarTypeBlockCharFifo(root RootMknod, path string, stat *types.Stat) error {
	mode := uint32(stat.Mode & 07777)
	if os.FileMode(stat.Mode)&os.ModeCharDevice != 0 {
		mode |= syscall.S_IFCHR
	} else if os.FileMode(stat.Mode)&os.ModeNamedPipe != 0 {
		mode |= syscall.S_IFIFO
	} else {
		mode |= syscall.S_IFBLK
	}

	return root.Mknod(path, mode, int(mkdev(stat.Devmajor, stat.Devminor)))
}
