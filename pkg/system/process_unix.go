//go:build linux || freebsd || darwin
// +build linux freebsd darwin

package system // import "github.com/docker/docker/pkg/system"

import (
	"bytes"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// IsProcessAlive returns true if process with a given pid is running.
func IsProcessAlive(pid int) bool {
	err := unix.Kill(pid, 0)
	if err == nil || err == unix.EPERM {
		return true
	}

	return false
}

// KillProcess force-stops a process.
func KillProcess(pid int) {
	unix.Kill(pid, unix.SIGKILL)
}

// IsProcessZombie return true if process has a state with "Z"
// http://man7.org/linux/man-pages/man5/proc.5.html
func IsProcessZombie(pid int) (bool, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if cols := bytes.SplitN(data, []byte(" "), 4); len(cols) >= 3 && string(cols[2]) == "Z" {
		return true, nil
	}
	return false, nil
}
