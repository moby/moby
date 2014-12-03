package graphdriver

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/docker/docker/pkg/archive"
)

type FsMagic uint64

const (
	FsMagicBtrfs = FsMagic(0x9123683E)
	FsMagicAufs  = FsMagic(0x61756673)
)

type InitFunc func(root string, options []string) (Driver, error)

// ProtoDriver defines the basic capabilities of a driver.
// This interface exists solely to be a minimum set of methods
// for client code which choose not to implement the entire Driver
// interface and use the NaiveDiffDriver wrapper constructor.
//
// Use of ProtoDriver directly by client code is not recommended.
type ProtoDriver interface {
	// String returns a string representation of this driver.
	String() string
	// Create creates a new, empty, filesystem layer with the
	// specified id and parent. Parent may be "".
	Create(id, parent string) error
	// Remove attempts to remove the filesystem layer with this id.
	Remove(id string) error
	// Get returns the mountpoint for the layered filesystem referred
	// to by this id. You can optionally specify a mountLabel or "".
	// Returns the absolute path to the mounted layered filesystem.
	Get(id, mountLabel string) (dir string, err error)
	// Put releases the system resources for the specified id,
	// e.g, unmounting layered filesystem.
	Put(id string)
	// Exists returns whether a filesystem layer with the specified
	// ID exists on this driver.
	Exists(id string) bool
	// Status returns a set of key-value pairs which give low
	// level diagnostic status about this driver.
	Status() [][2]string
	// Cleanup performs necessary tasks to release resources
	// held by the driver, e.g., unmounting all layered filesystems
	// known to this driver.
	Cleanup() error
}

// Driver is the interface for layered/snapshot file system drivers.
type Driver interface {
	ProtoDriver
	// Diff produces an archive of the changes between the specified
	// layer and its parent layer which may be "".
	Diff(id, parent string) (archive.Archive, error)
	// Changes produces a list of changes between the specified layer
	// and its parent layer. If parent is "", then all changes will be ADD changes.
	Changes(id, parent string) ([]archive.Change, error)
	// ApplyDiff extracts the changeset from the given diff into the
	// layer with the specified id and parent, returning the size of the
	// new layer in bytes.
	ApplyDiff(id, parent string, diff archive.ArchiveReader) (bytes int64, err error)
	// DiffSize calculates the changes between the specified id
	// and its parent and returns the size in bytes of the changes
	// relative to its base filesystem directory.
	DiffSize(id, parent string) (bytes int64, err error)
}

var (
	DefaultDriver string
	// All registred drivers
	drivers map[string]InitFunc
	// Slice of drivers that should be used in an order
	priority = []string{
		"aufs",
		"btrfs",
		"devicemapper",
		"vfs",
		// experimental, has to be enabled manually for now
		"overlay",
	}

	ErrNotSupported   = errors.New("driver not supported")
	ErrPrerequisites  = errors.New("prerequisites for driver not satisfied (wrong filesystem?)")
	ErrIncompatibleFS = fmt.Errorf("backing file system is unsupported for this graph driver")
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

func GetDriver(name, home string, options []string) (Driver, error) {
	if initFunc, exists := drivers[name]; exists {
		return initFunc(path.Join(home, name), options)
	}
	return nil, ErrNotSupported
}

func New(root string, options []string) (driver Driver, err error) {
	for _, name := range []string{os.Getenv("DOCKER_DRIVER"), DefaultDriver} {
		if name != "" {
			return GetDriver(name, root, options)
		}
	}

	// Check for priority drivers first
	for _, name := range priority {
		driver, err = GetDriver(name, root, options)
		if err != nil {
			if err == ErrNotSupported || err == ErrPrerequisites || err == ErrIncompatibleFS {
				continue
			}
			return nil, err
		}
		return driver, nil
	}

	// Check all registered drivers if no priority driver is found
	for _, initFunc := range drivers {
		if driver, err = initFunc(root, options); err != nil {
			if err == ErrNotSupported || err == ErrPrerequisites || err == ErrIncompatibleFS {
				continue
			}
			return nil, err
		}
		return driver, nil
	}
	return nil, fmt.Errorf("No supported storage backend found")
}
