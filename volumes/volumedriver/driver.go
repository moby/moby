package volumedriver

import (
	"errors"
	"fmt"
)

var (
	drivers        map[string]*RegisteredDriver
	DriverNotExist = errors.New("volume driver does not exist")
	DriverExist    = errors.New("volume driver exists")
)

type Driver interface {
	// Create creates a new volume at the specified path
	Create(path string) error
	// Remove attempts to remove the volume
	Remove(path string) error
	// Link tells the driver that a container is now using referenced volume
	Link(path, containerID string) error
	// Unlink tells the driver when a container has stopped using the referenced volume
	Unlink(path, containerID string) error
	// Exists returns weather the volume exists
	Exists(path string) bool
}

type InitFunc func(options map[string]string) (Driver, error)

type RegisteredDriver struct {
	New InitFunc
}

func init() {
	drivers = make(map[string]*RegisteredDriver)
}

func Register(name string, initFunc InitFunc) error {
	if _, exists := drivers[name]; exists {
		return fmt.Errorf("%v - %s", DriverExist, name)
	}
	drivers[name] = &RegisteredDriver{initFunc}
	return nil
}

// TODO: expose driver options
func NewDriver(name string) (Driver, error) {
	var opts map[string]string
	if d, exists := drivers[name]; exists {
		return d.New(opts)
	}
	return nil, fmt.Errorf("%v - %s", DriverNotExist, name)
}
