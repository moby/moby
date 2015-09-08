package volumedrivers

import (
	"errors"
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
)

// Opts are options that are currently passed as a string map
type Opts map[string]string

// Driver is the Volume Driver interface for adapters that creates and removes Volumes
type Driver interface {
	// Create a new driver adapter
	New(string, interface{}) Driver
	// Name returns the name of the volume driver.
	Name() string
	// Create makes a new volume with the given id.
	Create(string, Opts) (Volume, error)
	// Remove deletes the volume.
	Remove(Volume) error
	// Return a volume
	GetVolume(string) Volume
}

// Volume is the Volume interface that performs operations on Volumes
type Volume interface {
	// Name returns the name of the volume
	Name() string
	// DriverName returns the name of the driver which owns this volume.
	DriverName() string
	// Path returns the absolute path to the volume.
	Path() string
	// Mount mounts the volume and returns the absolute path to
	// where it can be consumed.
	Mount() (string, error)
	// Unmount unmounts the volume when it is no longer in use.
	Unmount() error
}

// InitFunc creates adapters using the driver initialization
type InitFunc func(root string) (Driver, error)

var (
	registeredDrivers = make(map[string]InitFunc)

	// ErrNotSupported is called when a driver is not supported
	ErrNotSupported = errors.New("driver not supported")
)

var drivers = &driverExtpoint{extensions: make(map[string]Driver)}

type driverExtpoint struct {
	extensions map[string]Driver
	sync.Mutex
}

// RegisterDriver initializes a driver
func RegisterDriver(name string, initFunc InitFunc) error {
	if _, exists := registeredDrivers[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}
	registeredDrivers[name] = initFunc

	return nil
}

// GetDriver returns a new driver instance from registered drivers
func GetDriver(name string) (Driver, error) {
	if initFunc, exists := registeredDrivers[name]; exists {
		return initFunc(name)
	}
	return nil, ErrNotSupported
}

// Lookup returns the driver associated with the given name. If a
// driver with the given name has not been registered it checks if
// there is a VolumeDriver plugin available with the given name.
func Lookup(name string) (Driver, error) {
	drivers.Lock()
	defer drivers.Unlock()
	ext, ok := drivers.extensions[name]
	if ok {
		return ext, nil
	}

	pl, err := plugins.Get(name, "")
	if err != nil {
		return nil, fmt.Errorf("Error looking up volume plugin %s: %v", name, err)
	}

	var driver Driver
	for _, driverName := range pl.Manifest.Implements {
		driver, err = GetDriver(driverName)
		if err != nil {
			logrus.Warnf("Error looking up volume driver %s: %v", driverName, err)
		} else if driver != nil {
			break
		}
	}
	if driver == nil {
		return nil, fmt.Errorf("Error looking up implemented volume drivers: %s", name)
	}

	driverAdapter := driver.New(name, pl.Client)
	if driverAdapter == nil {
		return nil, fmt.Errorf("Error looking up volume on adapter %s: %s", name, driver.Name())
	}

	drivers.extensions[name] = driverAdapter
	return driverAdapter, nil
}

// Register associates the given driver to the given name, checking if
// the name is already associated
func Register(extension Driver, name string) bool {
	drivers.Lock()
	defer drivers.Unlock()
	if name == "" {
		return false
	}
	_, exists := drivers.extensions[name]
	if exists {
		return false
	}
	drivers.extensions[name] = extension
	return true
}

// Unregister dissociates the name from it's driver, if the association exists.
func Unregister(name string) bool {
	drivers.Lock()
	defer drivers.Unlock()
	_, exists := drivers.extensions[name]
	if !exists {
		return false
	}
	delete(drivers.extensions, name)
	return true
}
