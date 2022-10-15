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

func checkPIDFileAlreadyExists(path string) error {
	pidByte, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	pid, err := strconv.Atoi(string(bytes.TrimSpace(pidByte)))
	if err == nil && process.Alive(pid) {
		return fmt.Errorf("pid file found, ensure docker is not running or delete %s", path)
	}
	return nil
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
	if err := checkPIDFileAlreadyExists(path); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}
