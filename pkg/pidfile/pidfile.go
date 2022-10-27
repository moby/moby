// Package pidfile provides structure and helper functions to create and remove
// PID file. A PID file is usually a file used to store the process ID of a
// running process.
package pidfile // import "github.com/docker/docker/pkg/pidfile"

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/docker/docker/pkg/system"
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
	if err == nil && processExists(pid) {
		return fmt.Errorf("pid file found, ensure docker is not running or delete %s", path)
	}
	return nil
}

// Write writes a "PID file" at the specified path. It returns an error if the
// file exists and contains a valid PID of a running process, or when failing
// to write the file.
func Write(path string) error {
	if err := checkPIDFileAlreadyExists(path); err != nil {
		return err
	}
	if err := system.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644)
}
