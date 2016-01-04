// +build windows

package windows

import (
	"github.com/docker/docker/daemon/execdriver"
)

// Update updates resource configs for a container.
func (d *Driver) Update(c *execdriver.Command) error {
	// Updating resource isn't supported on Windows
	// but we should return nil for enabling updating container
	return nil
}
