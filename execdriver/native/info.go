package native

import (
	"os"
	"path/filepath"
)

type info struct {
	ID     string
	driver *driver
}

// IsRunning is determined by looking for the
// .nspid file for a container.  If the file exists then the
// container is currently running
func (i *info) IsRunning() bool {
	p := filepath.Join(i.driver.root, "containers", i.ID, "root", ".nspid")
	if _, err := os.Stat(p); err == nil {
		return true
	}
	return false
}
