// +build linux osx freebsd openbsd

package archive

import (
	"os"
	"syscall"
)

func IsHardlink(fi os.FileInfo) bool {
	switch sys := fi.Sys().(type) {
	case *syscall.Stat_t:
		if fi.Mode().IsRegular() && sys.Nlink > 1 {
			return true
		}
	}
	return false
}
