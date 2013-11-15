package graphdriver

import (
	"fmt"
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/utils"
	"os"
	"path"
)

type InitFunc func(root string) (Driver, error)

type Driver interface {
	String() string

	Create(id, parent string) error
	Remove(id string) error

	Get(id string) (dir string, err error)
	Size(id string) (bytes int64, err error)

	Status() [][2]string

	Cleanup() error
}

type Differ interface {
	Diff(id string) (archive.Archive, error)
	Changes(id string) ([]archive.Change, error)
	ApplyDiff(id string, diff archive.Archive) error
}

var (
	// All registred drivers
	drivers map[string]InitFunc
	// Slice of drivers that should be used in an order
	priority = []string{
		"aufs",
		"devicemapper",
		"dummy",
	}
)

func init() {
	drivers = make(map[string]InitFunc)
}

func Register(name string, initFunc InitFunc) error {
	if _, exists := drivers[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}
	drivers[name] = initFunc

	return nil
}

func GetDriver(name, home string) (Driver, error) {
	if initFunc, exists := drivers[name]; exists {
		return initFunc(path.Join(home, name))
	}
	return nil, fmt.Errorf("No such driver: %s", name)
}

func New(root string) (Driver, error) {
	var driver Driver
	var lastError error
	// Use environment variable DOCKER_DRIVER to force a choice of driver
	if name := os.Getenv("DOCKER_DRIVER"); name != "" {
		return GetDriver(name, root)
	}
	// Check for priority drivers first
	for _, name := range priority {
		driver, lastError = GetDriver(name, root)
		if lastError != nil {
			utils.Debugf("Error loading driver %s: %s", name, lastError)
			continue
		}
		return driver, nil
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
