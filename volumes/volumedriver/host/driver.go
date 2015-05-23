package host

import (
	"os"

	"github.com/docker/docker/volumes/volumedriver"
)

const DriverName = "host"

func init() {
	volumedriver.Register(DriverName, Init)
}

func Init(_ map[string]string) (volumedriver.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Create(path string) error {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return os.MkdirAll(path, 0700)
	}
	return err
}

func (d *Driver) Remove(path string) error {
	// don't remove volumes from this driver
	return nil
}

func (d *Driver) Link(path, containerID string) error {
	return nil
}

func (d *Driver) Unlink(path, containerID string) error {
	return nil
}

func (d *Driver) Exists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}
