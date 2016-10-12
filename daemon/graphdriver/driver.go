package graphdriver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/vbatts/tar-split/tar/storage"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/plugingetter"
)

// FsMagic unsigned id of the filesystem in use.
type FsMagic uint32

const (
	// FsMagicUnsupported is a predefined constant value other than a valid filesystem id.
	FsMagicUnsupported = FsMagic(0x00000000)
)

var (
	// All registered drivers
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
	// CreateReadWrite creates a new, empty filesystem layer that is ready
	// to be used as the storage for a container.
	CreateReadWrite(id, parent, mountLabel string, storageOpt map[string]string) error
	// Create creates a new, empty, filesystem layer with the
	// specified id and parent and mountLabel. Parent and mountLabel may be "".
	Create(id, parent, mountLabel string, storageOpt map[string]string) error
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

// DiffGetterDriver is the interface for layered file system drivers that
// provide a specialized function for getting file contents for tar-split.
type DiffGetterDriver interface {
	Driver
	// DiffGetter returns an interface to efficiently retrieve the contents
	// of files in a layer.
	DiffGetter(id string) (FileGetCloser, error)
}

// FileGetCloser extends the storage.FileGetter interface with a Close method
// for cleaning up.
type FileGetCloser interface {
	storage.FileGetter
	// Close cleans up any resources associated with the FileGetCloser.
	Close() error
}

// Checker makes checks on specified filesystems.
type Checker interface {
	// IsMounted returns true if the provided path is mounted for the specific checker
	IsMounted(path string) bool
}

func init() {
	drivers = make(map[string]InitFunc)
}

// Register registers an InitFunc for the driver.
func Register(name string, initFunc InitFunc) error {
	if _, exists := drivers[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}
	drivers[name] = initFunc

	return nil
}

// GetDriver initializes and returns the registered driver
func GetDriver(name, home string, options []string, uidMaps, gidMaps []idtools.IDMap, pg plugingetter.PluginGetter) (Driver, error) {
	if initFunc, exists := drivers[name]; exists {
		return initFunc(filepath.Join(home, name), options, uidMaps, gidMaps)
	}
	if pluginDriver, err := lookupPlugin(name, home, options, pg); err == nil {
		return pluginDriver, nil
	}
	logrus.Errorf("Failed to GetDriver graph %s %s", name, home)
	return nil, ErrNotSupported
}

// getBuiltinDriver initializes and returns the registered driver, but does not try to load from plugins
func getBuiltinDriver(name, home string, options []string, uidMaps, gidMaps []idtools.IDMap) (Driver, error) {
	if initFunc, exists := drivers[name]; exists {
		return initFunc(filepath.Join(home, name), options, uidMaps, gidMaps)
	}
	logrus.Errorf("Failed to built-in GetDriver graph %s %s", name, home)
	return nil, ErrNotSupported
}

// New creates the driver and initializes it at the specified root.
func New(root string, name string, options []string, uidMaps, gidMaps []idtools.IDMap, pg plugingetter.PluginGetter) (Driver, error) {
	if name != "" {
		logrus.Debugf("[graphdriver] trying provided driver: %s", name) // so the logs show specified driver
		return GetDriver(name, root, options, uidMaps, gidMaps, pg)
	}

	// Guess for prior driver
	driversMap := scanPriorDrivers(root)
	for _, name := range priority {
		if name == "vfs" {
			// don't use vfs even if there is state present.
			continue
		}
		if _, prior := driversMap[name]; prior {
			// of the state found from prior drivers, check in order of our priority
			// which we would prefer
			driver, err := getBuiltinDriver(name, root, options, uidMaps, gidMaps)
			if err != nil {
				// unlike below, we will return error here, because there is prior
				// state, and now it is no longer supported/prereq/compatible, so
				// something changed and needs attention. Otherwise the daemon's
				// images would just "disappear".
				logrus.Errorf("[graphdriver] prior storage driver %s failed: %s", name, err)
				return nil, err
			}

			// abort starting when there are other prior configured drivers
			// to ensure the user explicitly selects the driver to load
			if len(driversMap)-1 > 0 {
				var driversSlice []string
				for name := range driversMap {
					driversSlice = append(driversSlice, name)
				}

				return nil, fmt.Errorf("%s contains several valid graphdrivers: %s; Please cleanup or explicitly choose storage driver (-s <DRIVER>)", root, strings.Join(driversSlice, ", "))
			}

			logrus.Infof("[graphdriver] using prior storage driver: %s", name)
			return driver, nil
		}
	}

	// Check for priority drivers first
	for _, name := range priority {
		driver, err := getBuiltinDriver(name, root, options, uidMaps, gidMaps)
		if err != nil {
			if isDriverNotSupported(err) {
				continue
			}
			return nil, err
		}
		return driver, nil
	}

	// Check all registered drivers if no priority driver is found
	for name, initFunc := range drivers {
		driver, err := initFunc(filepath.Join(root, name), options, uidMaps, gidMaps)
		if err != nil {
			if isDriverNotSupported(err) {
				continue
			}
			return nil, err
		}
		return driver, nil
	}
	return nil, fmt.Errorf("No supported storage backend found")
}

// isDriverNotSupported returns true if the error initializing
// the graph driver is a non-supported error.
func isDriverNotSupported(err error) bool {
	return err == ErrNotSupported || err == ErrPrerequisites || err == ErrIncompatibleFS
}

// scanPriorDrivers returns an un-ordered scan of directories of prior storage drivers
func scanPriorDrivers(root string) map[string]bool {
	driversMap := make(map[string]bool)

	for driver := range drivers {
		p := filepath.Join(root, driver)
		if _, err := os.Stat(p); err == nil && driver != "vfs" {
			driversMap[driver] = true
		}
	}
	return driversMap
}
