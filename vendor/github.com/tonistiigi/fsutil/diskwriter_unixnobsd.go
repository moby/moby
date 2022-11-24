//go:build !windows && !freebsd
// +build !windows,!freebsd

package fsutil

import (
	"github.com/tonistiigi/fsutil/types"
	"golang.org/x/sys/unix"
)

func createSpecialFile(path string, mode uint32, stat *types.Stat) error {
	return unix.Mknod(path, mode, mkdev(stat.Devmajor, stat.Devminor))
}

func mkdev(major int64, minor int64) int {
	return int(unix.Mkdev(uint32(major), uint32(minor)))
}
