//go:build !windows
// +build !windows

package process

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"golang.org/x/sys/unix"
)

// Alive returns true if process with a given pid is running.
func Alive(pid int) bool {
	switch runtime.GOOS {
	case "darwin":
		// OS X does not have a proc filesystem. Use kill -0 pid to judge if the
		// process exists. From KILL(2): https://www.freebsd.org/cgi/man.cgi?query=kill&sektion=2&manpath=OpenDarwin+7.2.1
		//
		// Sig may be one of the signals specified in sigaction(2) or it may
		// be 0, in which case error checking is performed but no signal is
		// actually sent. This can be used to check the validity of pid.
		err := unix.Kill(pid, 0)

		// Either the PID was found (no error) or we get an EPERM, which means
		// the PID exists, but we don't have permissions to signal it.
		return err == nil || err == unix.EPERM
	default:
		_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid)))
		return err == nil
	}
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
