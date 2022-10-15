//go:build linux || freebsd || darwin
// +build linux freebsd darwin

package process

import (
	"bytes"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Alive returns true if process with a given pid is running.
func Alive(pid int) bool {
	err := unix.Kill(pid, 0)
	if err == nil || err == unix.EPERM {
		return true
	}

	return false
}

// Kill force-stops a process.
func Kill(pid int) error {
	err := unix.Kill(pid, unix.SIGKILL)
	if err != nil && err != unix.ESRCH {
		return err
	}
	return nil
}

// Zombie return true if process has a state with "Z"
// http://man7.org/linux/man-pages/man5/proc.5.html
func Zombie(pid int) (bool, error) {
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
