//go:build linux || freebsd || darwin
// +build linux freebsd darwin

package system // import "github.com/docker/docker/pkg/system"

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// IsProcessAlive returns true if process with a given pid is running.
func IsProcessAlive(pid int) bool {
	err := unix.Kill(pid, syscall.Signal(0))
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
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	dataBytes, err := os.ReadFile(statPath)
	if err != nil {
		return false, err
	}
	data := string(dataBytes)
	sdata := strings.SplitN(data, " ", 4)
	if len(sdata) >= 3 && sdata[2] == "Z" {
		return true, nil
	}

	return false, nil
}
