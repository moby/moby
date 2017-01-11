// +build darwin

package pidfile

import (
	"syscall"
)

func processExists(pid int) bool {
	// OS X does not have a proc filesystem.
	// Use kill -0 pid to judge if the process exists.
	err := syscall.Kill(pid, 0)
	return err == nil
}
