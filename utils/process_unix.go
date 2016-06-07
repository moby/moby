// +build linux freebsd solaris

package utils

import (
	"syscall"
)

// IsProcessAlive returns true if process with a given pid is running.
func IsProcessAlive(pid int) bool {
	err := syscall.Kill(pid, syscall.Signal(0))
	if err == nil || err == syscall.EPERM {
		return true
	}

	return false
}

// KillProcess force-stops a process.
func KillProcess(pid int) {
	syscall.Kill(pid, syscall.SIGKILL)
}
