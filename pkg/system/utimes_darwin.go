package system

import "syscall"

// LUtimesNano is not supported by darwin platform.
func LUtimesNano(path string, ts []syscall.Timespec) error {
	return ErrNotSupportedPlatform
}

// UtimesNano is used to change access and modification time of path.
// it can't be used for symbol link file.
func UtimesNano(path string, ts []syscall.Timespec) error {
	return syscall.UtimesNano(path, ts)
}
