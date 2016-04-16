package graphdriver

import (
	"os"

	"github.com/docker/docker/pkg/system"
)

// setupDaemonRoot initializes the root directory with the right mode.
func setupDaemonRoot(rootDir string, rootUID, rootGID int) error {
	// Create the root directory if it doesn't exists
	if err := system.MkdirAll(rootDir, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}
