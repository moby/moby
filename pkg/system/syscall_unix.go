// +build linux freebsd

package system

import "syscall"

// UnmountWithSyscall is a platform-specific helper function to call
// the unmount syscall.
func UnmountWithSyscall(dest string) {
	syscall.Unmount(dest, 0)
}
