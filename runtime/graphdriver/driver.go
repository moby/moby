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
	Put(id string)
	Exists(id string) bool

	Status() [][2]string

	Cleanup() error
}

type Differ interface {
	Diff(id string) (archive.Archive, error)
	Changes(id string) ([]archive.Change, error)
	ApplyDiff(id string, diff archive.ArchiveReader) error
	DiffSize(id string) (bytes int64, err error)
}

var (
	DefaultDriver string
	// All registred drivers
	drivers map[string]InitFunc
	// Slice of drivers that should be used in an order
	priority = []string{
		"aufs",
		"devicemapper",
		"vfs",
		// experimental, has to be enabled manually for now
		"btrfs",
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

func New(root string) (driver Driver, err error) {
	for _, name := range []string{os.Getenv("DOCKER_DRIVER"), DefaultDriver} {
		if name != "" {
			return GetDriver(name, root)
		}
	}

	// Check for priority drivers first
	for _, name := range priority {
		if driver, err = GetDriver(name, root); err != nil {
			utils.Debugf("Error loading driver %s: %s", name, err)
			continue
		}
		return driver, nil
	}

	// Check all registered drivers if no priority driver is found
	for _, initFunc := range drivers {
		if driver, err = initFunc(root); err != nil {
			continue
		}
		return driver, nil
	}
	return nil, err
}
