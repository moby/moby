package system

import (
	"syscall"
)

func GetLastAccess(stat *syscall.Stat_t) syscall.Timespec {
	return stat.Atim
}

func GetLastModification(stat *syscall.Stat_t) syscall.Timespec {
	return stat.Mtim
}
