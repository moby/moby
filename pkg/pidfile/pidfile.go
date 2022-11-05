// Package pidfile provides structure and helper functions to create and remove
// PID file. A PID file is usually a file used to store the process ID of a
// running process.
package pidfile // import "github.com/docker/docker/pkg/pidfile"

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"github.com/docker/docker/pkg/process"
)

// Read reads the "PID file" at path, and returns the PID if it contains a
// valid PID of a running process, or 0 otherwise. It returns an error when
// failing to read the file, or if the file doesn't exist, but malformed content
// is ignored. Consumers should therefore check if the returned PID is a non-zero
// value before use.
func Read(path string) (pid int, err error) {
	pidByte, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err = strconv.Atoi(string(bytes.TrimSpace(pidByte)))
	if err != nil {
		return 0, nil
	}
	if pid != 0 && process.Alive(pid) {
		return pid, nil
	}
	return 0, nil
}

// Write writes a "PID file" at the specified path. It returns an error if the
// file exists and contains a valid PID of a running process, or when failing
// to write the file.
func Write(path string, pid int) error {
	if pid < 1 {
		// We might be running as PID 1 when running docker-in-docker,
		// but 0 or negative PIDs are not acceptable.
		return fmt.Errorf("invalid PID (%d): only positive PIDs are allowed", pid)
	}
	oldPID, err := Read(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if oldPID != 0 {
		return fmt.Errorf("process with PID %d is still running", oldPID)
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}
