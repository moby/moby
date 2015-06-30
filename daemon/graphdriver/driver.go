package graphdriver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/pkg/archive"
)

type FsMagic uint32

const (
	FsMagicUnsupported = FsMagic(0x00000000)
)

var (
	DefaultDriver string
	// All registred drivers
	drivers map[string]InitFunc

	ErrNotSupported                 = errors.New("driver not supported")
	ErrPrerequisites                = errors.New("prerequisites for driver not satisfied (wrong filesystem?)")
	ErrIncompatibleFS               = fmt.Errorf("backing file system is unsupported for this graph driver")
	ErrDeviceMapperWithStaticDocker = fmt.Errorf("devicemapper storage driver cannot reliably be used with a statically linked docker binary: please either pick a different storage driver, install a dynamically linked docker binary, or force this unreliable setup anyway by specifying --storage-driver=devicemapper")
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
	Put(id string) error
	// Exists returns whether a filesystem layer with the specified
	// ID exists on this driver.
	Exists(id string) bool
	// Status returns a set of key-value pairs which give low
	// level diagnostic status about this driver.
	Status() [][2]string
	// Returns a set of key-value pairs which give low level information
	// about the image/container driver is managing.
	GetMetadata(id string) (map[string]string, error)
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
	ApplyDiff(id, parent string, diff archive.ArchiveReader) (size int64, err error)
	// DiffSize calculates the changes between the specified id
	// and its parent and returns the size in bytes of the changes
	// relative to its base filesystem directory.
	DiffSize(id, parent string) (size int64, err error)
}

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
		return initFunc(filepath.Join(home, name), options)
	}
	return nil, ErrNotSupported
}

func New(root string, options []string) (driver Driver, err error) {
	for _, name := range []string{os.Getenv("DOCKER_DRIVER"), DefaultDriver} {
		if name != "" {
			logrus.Debugf("[graphdriver] trying provided driver %q", name) // so the logs show specified driver
			return GetDriver(name, root, options)
		}
	}

	// Guess for prior driver
	priorDriver, err := scanPriorDrivers(root)
	if err != nil {
		return nil, err
	}

	if len(priorDriver) != 0 {
		// Do not allow devicemapper when it's not explicit and the Docker binary was built statically.
		if staticWithDeviceMapper(priorDriver) {
			return nil, ErrDeviceMapperWithStaticDocker
		}

		driver, err = GetDriver(priorDriver, root, options)
		if err != nil {
			// unlike below, we will return error here, because there is prior
			// state, and now it is no longer supported/prereq/compatible, so
			// something changed and needs attention. Otherwise the daemon's
			// images would just "disappear".
			logrus.Errorf("[graphdriver] prior storage driver %q failed: %s", priorDriver, err)
			return nil, err
		}
		logrus.Infof("[graphdriver] using prior storage driver %q", priorDriver)
		return driver, nil
	}

	// Check for priority drivers first
	for _, name := range priority {
		if staticWithDeviceMapper(name) {
			continue
		}
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
	for name, initFunc := range drivers {
		if staticWithDeviceMapper(name) {
			continue
		}
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

// scanPriorDrivers returns a previosly used driver.
// it returns an error when there are several drivers scanned.
func scanPriorDrivers(root string) (string, error) {
	var priorDrivers []string
	for driver := range drivers {
		p := filepath.Join(root, driver)
		if _, err := os.Stat(p); err == nil && driver != "vfs" {
			priorDrivers = append(priorDrivers, driver)
		}
	}

	if len(priorDrivers) > 1 {
		return "", multipleDriversError(root, priorDrivers)
	}

	if len(priorDrivers) == 0 {
		return "", nil
	}
	return priorDrivers[0], nil
}

func multipleDriversError(root string, drivers []string) error {
	return fmt.Errorf("%q contains several graphdrivers: %s; Please cleanup or explicitly choose storage driver (--storage-driver <DRIVER>)", root, strings.Join(drivers, ", "))
}

func staticWithDeviceMapper(name string) bool {
	return name == "devicemapper" && dockerversion.IAMSTATIC == "true"
}
