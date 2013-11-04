package graphdriver

import (
	"fmt"
)

type InitFunc func(root string) (Driver, error)

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

type Image interface {
	Layers() ([]string, error)
}

type Driver interface {
	//	Create(img *Image) error
	//	Delete(img *Image) error
	Mount(img Image, root string) error
	Unmount(root string) error
	Mounted(root string) (bool, error)
	//	UnmountAll(img *Image) error
	//	Changes(img *Image, dest string) ([]Change, error)
	//	Layer(img *Image, dest string) (Archive, error)
}
