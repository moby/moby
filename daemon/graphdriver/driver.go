package graphdriver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
)

// FsMagic unsigned id of the filesystem in use.
type FsMagic uint32

const (
	// FsMagicUnsupported is a predifined contant value other than a valid filesystem id.
	FsMagicUnsupported = FsMagic(0x00000000)
)

var (
	// DefaultDriver if a storage driver is not specified.
	DefaultDriver string
	// All registred drivers
	drivers map[string]InitFunc

	// ErrNotSupported returned when driver is not supported.
	ErrNotSupported = errors.New("driver not supported")
	// ErrPrerequisites retuned when driver does not meet prerequisites.
	ErrPrerequisites = errors.New("prerequisites for driver not satisfied (wrong filesystem?)")
	// ErrIncompatibleFS returned when file system is not supported.
	ErrIncompatibleFS = fmt.Errorf("backing file system is unsupported for this graph driver")
)

// InitFunc initializes the storage driver.
type InitFunc func(root string, options []string, uidMaps, gidMaps []idtools.IDMap) (Driver, error)

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
	// The archive.Reader must be an uncompressed stream.
	ApplyDiff(id, parent string, diff archive.Reader) (size int64, err error)
	// DiffSize calculates the changes between the specified id
	// and its parent and returns the size in bytes of the changes
	// relative to its base filesystem directory.
	DiffSize(id, parent string) (size int64, err error)
}

func init() {
	drivers = make(map[string]InitFunc)
}

// Register registers a InitFunc for the driver.
func Register(name string, initFunc InitFunc) error {
	if _, exists := drivers[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}
	drivers[name] = initFunc

	return nil
}

// GetDriver initializes and returns the registered driver
func GetDriver(name, home string, options []string, uidMaps, gidMaps []idtools.IDMap) (Driver, error) {
	if initFunc, exists := drivers[name]; exists {
		return initFunc(filepath.Join(home, name), options, uidMaps, gidMaps)
	}
	if pluginDriver, err := lookupPlugin(name, home, options); err == nil {
		return pluginDriver, nil
	}
	logrus.Errorf("Failed to GetDriver graph %s %s", name, home)
	return nil, ErrNotSupported
}

// getBuiltinDriver initalizes and returns the registered driver, but does not try to load from plugins
func getBuiltinDriver(name, home string, options []string, uidMaps, gidMaps []idtools.IDMap) (Driver, error) {
	if initFunc, exists := drivers[name]; exists {
		return initFunc(filepath.Join(home, name), options, uidMaps, gidMaps)
	}
	logrus.Errorf("Failed to built-in GetDriver graph %s %s", name, home)
	return nil, ErrNotSupported
}

// New creates the driver and initializes it at the specified root.
func New(root string, options []string, uidMaps, gidMaps []idtools.IDMap) (driver Driver, err error) {
	for _, name := range []string{os.Getenv("DOCKER_DRIVER"), DefaultDriver} {
		if name != "" {
			logrus.Debugf("[graphdriver] trying provided driver %q", name) // so the logs show specified driver
			return GetDriver(name, root, options, uidMaps, gidMaps)
		}
	}

	// Guess for prior driver
	priorDrivers := scanPriorDrivers(root)
	for _, name := range priority {
		if name == "vfs" {
			// don't use vfs even if there is state present.
			continue
		}
		for _, prior := range priorDrivers {
			// of the state found from prior drivers, check in order of our priority
			// which we would prefer
			if prior == name {
				driver, err = getBuiltinDriver(name, root, options, uidMaps, gidMaps)
				if err != nil {
					// unlike below, we will return error here, because there is prior
					// state, and now it is no longer supported/prereq/compatible, so
					// something changed and needs attention. Otherwise the daemon's
					// images would just "disappear".
					logrus.Errorf("[graphdriver] prior storage driver %q failed: %s", name, err)
					return nil, err
				}
				if err := checkPriorDriver(name, root); err != nil {
					return nil, err
				}
				logrus.Infof("[graphdriver] using prior storage driver %q", name)
				return driver, nil
			}
		}
	}

	// Check for priority drivers first
	for _, name := range priority {
		driver, err = getBuiltinDriver(name, root, options, uidMaps, gidMaps)
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
		if driver, err = initFunc(root, options, uidMaps, gidMaps); err != nil {
			if err == ErrNotSupported || err == ErrPrerequisites || err == ErrIncompatibleFS {
				continue
			}
			return nil, err
		}
		return driver, nil
	}
	return nil, fmt.Errorf("No supported storage backend found")
}

// scanPriorDrivers returns an un-ordered scan of directories of prior storage drivers
func scanPriorDrivers(root string) []string {
	priorDrivers := []string{}
	for driver := range drivers {
		p := filepath.Join(root, driver)
		if _, err := os.Stat(p); err == nil && driver != "vfs" {
			priorDrivers = append(priorDrivers, driver)
		}
	}
	return priorDrivers
}

func checkPriorDriver(name, root string) error {
	priorDrivers := []string{}
	for _, prior := range scanPriorDrivers(root) {
		if prior != name && prior != "vfs" {
			if _, err := os.Stat(filepath.Join(root, prior)); err == nil {
				priorDrivers = append(priorDrivers, prior)
			}
		}
	}

	if len(priorDrivers) > 0 {

		return fmt.Errorf("%q contains other graphdrivers: %s; Please cleanup or explicitly choose storage driver (-s <DRIVER>)", root, strings.Join(priorDrivers, ","))
	}
	return nil
}
