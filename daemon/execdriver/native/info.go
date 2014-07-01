package native

import (
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
	return false
}
