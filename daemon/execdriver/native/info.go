// +build linux,cgo

package native

import (
	"os"
	"path/filepath"

	"github.com/docker/libcontainer"
)

type info struct {
	ID     string
	driver *driver
}

// IsRunning is determined by looking for the
// pid file for a container.  If the file exists then the
// container is currently running
func (i *info) IsRunning() bool {
	if _, err := libcontainer.GetState(filepath.Join(i.driver.root, i.ID)); err == nil {
		return true
	}
	// TODO: Remove this part for version 1.2.0
	// This is added only to ensure smooth upgrades from pre 1.1.0 to 1.1.0
	if _, err := os.Stat(filepath.Join(i.driver.root, i.ID, "pid")); err == nil {
		return true
	}
	return false
}
