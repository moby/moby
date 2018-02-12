// +build !windows

package fs

import (
	"os"
	"syscall"
)

func getLinkInfo(fi os.FileInfo) (uint64, bool) {
	s, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}

	// Ino is uint32 on bsd, uint64 on darwin/linux/solaris
	return uint64(s.Ino), !fi.IsDir() && s.Nlink > 1 // nolint: unconvert
}
