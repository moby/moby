// +build !linux

package system

import "syscall"

func GetLastAccess(stat *syscall.Stat_t) syscall.Timespec {
	return stat.Atimespec
}

func GetLastModification(stat *syscall.Stat_t) syscall.Timespec {
	return stat.Mtimespec
}
