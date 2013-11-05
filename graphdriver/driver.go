package graphdriver

import (
	"fmt"
	"github.com/dotcloud/docker/archive"
)

type InitFunc func(root string) (Driver, error)

type Dir interface {
	ID() string
	Path() string
	Parent() (Dir, error)
}

type Driver interface {
	OnCreate(dir Dir, layer archive.Archive) error
	OnRemove(dir Dir) error

	OnMount(dir Dir, dest string) error
	OnUnmount(dest string) error
	Mounted(dest string) (bool, error)

	Layer(dir Dir, dest string) (archive.Archive, error)

	Cleanup() error
}

var (
	// All registred drivers
	drivers map[string]InitFunc
	// Slice of drivers that should be used in an order
	priority = []string{
		"aufs",
		"devicemapper",
	}
)

func Register(name string, initFunc InitFunc) error {
	if _, exists := drivers[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}
	drivers[name] = initFunc

	return nil
}

func New(root string) (Driver, error) {
	var driver Driver
	var lastError error
	// Check for priority drivers first
	for _, name := range priority {
		if initFunc, exists := drivers[name]; exists {
			driver, lastError = initFunc(root)
			if lastError != nil {
				continue
			}
			return driver, nil
		}
	}

	// Check all registered drivers if no priority driver is found
	for _, initFunc := range drivers {
		driver, lastError = initFunc(root)
		if lastError != nil {
			continue
		}
		return driver, nil
	}
	return nil, lastError
}
