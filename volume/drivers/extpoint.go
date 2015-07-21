package volumedrivers

import (
	"fmt"
	"sync"

	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/volume"
)

// currently created by hand. generation tool would generate this like:
// $ extpoint-gen Driver > volume/extpoint.go

var drivers = &driverExtpoint{extensions: make(map[string]volume.Driver)}

type driverExtpoint struct {
	extensions map[string]volume.Driver
	sync.Mutex
}

// Register associates the given driver to the given name, checking if
// the name is already associated
func Register(extension volume.Driver, name string) bool {
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

// Lookup returns the driver associated with the given name. If a
// driver with the given name has not been registered it checks if
// there is a VolumeDriver plugin available with the given name.
func Lookup(name string) (volume.Driver, error) {
	drivers.Lock()
	defer drivers.Unlock()
	ext, ok := drivers.extensions[name]
	if ok {
		return ext, nil
	}
	pl, err := plugins.Get(name, "VolumeDriver")
	if err != nil {
		return nil, fmt.Errorf("Error looking up volume plugin %s: %v", name, err)
	}

	d := NewVolumeDriver(name, pl.Client)
	drivers.extensions[name] = d
	return d, nil
}
