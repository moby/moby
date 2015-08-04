// +build windows

package windows

import "github.com/docker/docker/daemon/execdriver"

type info struct {
	ID     string
	driver *Driver
}

// Info implements the exec driver Driver interface.
func (d *Driver) Info(id string) execdriver.Info {
	return &info{
		ID:     id,
		driver: d,
	}
}

func (i *info) IsRunning() bool {
	var running bool
	running = true // TODO Need an HCS API
	return running
}
