// +build !windows

package archive

import (
	"os"
	"syscall"
)

func (sf SeenFiles) Add(fi os.FileInfo) {
	switch sys := fi.Sys().(type) {
	case *syscall.Stat_t:
		sf[uint64(sys.Ino)] = fi.Name()
	}
}

func (sf SeenFiles) Include(fi os.FileInfo) string {
	switch sys := fi.Sys().(type) {
	case *syscall.Stat_t:
		return sf[uint64(sys.Ino)]
	}
	return ""
}
