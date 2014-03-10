// +build !linux !amd64

package archive

import "syscall"

func getLastAccess(stat *syscall.Stat_t) syscall.Timespec {
	return syscall.Timespec{}
}

func getLastModification(stat *syscall.Stat_t) syscall.Timespec {
	return syscall.Timespec{}
}

func LUtimesNano(path string, ts []syscall.Timespec) error {
	return ErrNotImplemented
}

func UtimesNano(path string, ts []syscall.Timespec) error {
	return ErrNotImplemented
}
