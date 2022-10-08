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

// PIDFile is a file used to store the process ID of a running process.
type PIDFile struct {
	path string
}

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

// New creates a PIDfile using the specified path.
func New(path string) (*PIDFile, error) {
	if err := checkPIDFileAlreadyExists(path); err != nil {
		return nil, err
	}
	// Note MkdirAll returns nil if a directory already exists
	if err := system.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		return nil, err
	}

	return &PIDFile{path: path}, nil
}

// Remove removes the PIDFile.
func (file PIDFile) Remove() error {
	return os.Remove(file.path)
}
