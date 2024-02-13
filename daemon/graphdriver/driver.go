package graphdriver // import "github.com/docker/docker/daemon/graphdriver"

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/pkg/errors"
	"github.com/vbatts/tar-split/tar/storage"
)

// FsMagic unsigned id of the filesystem in use.
type FsMagic uint32

const (
	// FsMagicUnsupported is a predefined constant value other than a valid filesystem id.
	FsMagicUnsupported = FsMagic(0x00000000)
)

// All registered drivers
var drivers map[string]InitFunc

// CreateOpts contains optional arguments for Create() and CreateReadWrite()
// methods.
type CreateOpts struct {
	MountLabel string
	StorageOpt map[string]string
}

// InitFunc initializes the storage driver.
type InitFunc func(root string, options []string, idMap idtools.IdentityMapping) (Driver, error)

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
	// to be used as the storage for a container. Additional options can
	// be passed in opts. parent may be "" and opts may be nil.
	CreateReadWrite(id, parent string, opts *CreateOpts) error
	// Create creates a new, empty, filesystem layer with the
	// specified id and parent and options passed in opts. Parent
	// may be "" and opts may be nil.
	Create(id, parent string, opts *CreateOpts) error
	// Remove attempts to remove the filesystem layer with this id.
	Remove(id string) error
	// Get returns the mountpoint for the layered filesystem referred
	// to by this id. You can optionally specify a mountLabel or "".
	// Returns the absolute path to the mounted layered filesystem.
	Get(id, mountLabel string) (fs string, err error)
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

// DiffDriver is the interface to use to implement graph diffs
type DiffDriver interface {
	// Diff produces an archive of the changes between the specified
	// layer and its parent layer which may be "".
	Diff(id, parent string) (io.ReadCloser, error)
	// Changes produces a list of changes between the specified layer
	// and its parent layer. If parent is "", then all changes will be ADD changes.
	Changes(id, parent string) ([]archive.Change, error)
	// ApplyDiff extracts the changeset from the given diff into the
	// layer with the specified id and parent, returning the size of the
	// new layer in bytes.
	// The archive.Reader must be an uncompressed stream.
	ApplyDiff(id, parent string, diff io.Reader) (size int64, err error)
	// DiffSize calculates the changes between the specified id
	// and its parent and returns the size in bytes of the changes
	// relative to its base filesystem directory.
	DiffSize(id, parent string) (size int64, err error)
}

// Driver is the interface for layered/snapshot file system drivers.
type Driver interface {
	ProtoDriver
	DiffDriver
}

// Capabilities defines a list of capabilities a driver may implement.
// These capabilities are not required; however, they do determine how a
// graphdriver can be used.
type Capabilities struct {
	// Flags that this driver is capable of reproducing exactly equivalent
	// diffs for read-only layers. If set, clients can rely on the driver
	// for consistent tar streams, and avoid extra processing to account
	// for potential differences (eg: the layer store's use of tar-split).
	ReproducesExactDiffs bool
}

// CapabilityDriver is the interface for layered file system drivers that
// can report on their Capabilities.
type CapabilityDriver interface {
	Capabilities() Capabilities
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
		return errors.Errorf("name already registered %s", name)
	}
	drivers[name] = initFunc

	return nil
}

// GetDriver initializes and returns the registered driver
func GetDriver(name string, pg plugingetter.PluginGetter, config Options) (Driver, error) {
	if initFunc, exists := drivers[name]; exists {
		return initFunc(filepath.Join(config.Root, name), config.DriverOptions, config.IDMap)
	}

	pluginDriver, err := lookupPlugin(name, pg, config)
	if err == nil {
		return pluginDriver, nil
	}
	log.G(context.TODO()).WithError(err).WithField("driver", name).WithField("home-dir", config.Root).Error("Failed to GetDriver graph")
	return nil, ErrNotSupported
}

// getBuiltinDriver initializes and returns the registered driver, but does not try to load from plugins
func getBuiltinDriver(name, home string, options []string, idMap idtools.IdentityMapping) (Driver, error) {
	if initFunc, exists := drivers[name]; exists {
		return initFunc(filepath.Join(home, name), options, idMap)
	}
	log.G(context.TODO()).Errorf("Failed to built-in GetDriver graph %s %s", name, home)
	return nil, ErrNotSupported
}

// Options is used to initialize a graphdriver
type Options struct {
	Root                string
	DriverOptions       []string
	IDMap               idtools.IdentityMapping
	ExperimentalEnabled bool
}

// New creates the driver and initializes it at the specified root.
func New(name string, pg plugingetter.PluginGetter, config Options) (Driver, error) {
	ctx := context.TODO()
	if name != "" {
		log.G(ctx).Infof("[graphdriver] trying configured driver: %s", name)
		if err := checkRemoved(name); err != nil {
			return nil, err
		}
		return GetDriver(name, pg, config)
	}

	// Guess for prior driver
	driversMap := scanPriorDrivers(config.Root)
	priorityList := strings.Split(priority, ",")
	log.G(ctx).Debugf("[graphdriver] priority list: %v", priorityList)
	for _, name := range priorityList {
		if _, prior := driversMap[name]; prior {
			// of the state found from prior drivers, check in order of our priority
			// which we would prefer
			driver, err := getBuiltinDriver(name, config.Root, config.DriverOptions, config.IDMap)
			if err != nil {
				// unlike below, we will return error here, because there is prior
				// state, and now it is no longer supported/prereq/compatible, so
				// something changed and needs attention. Otherwise the daemon's
				// images would just "disappear".
				log.G(ctx).Errorf("[graphdriver] prior storage driver %s failed: %s", name, err)
				return nil, err
			}

			// abort starting when there are other prior configured drivers
			// to ensure the user explicitly selects the driver to load
			if len(driversMap) > 1 {
				var driversSlice []string
				for name := range driversMap {
					driversSlice = append(driversSlice, name)
				}

				err = errors.Errorf("%s contains several valid graphdrivers: %s; cleanup or explicitly choose storage driver (-s <DRIVER>)", config.Root, strings.Join(driversSlice, ", "))
				log.G(ctx).Errorf("[graphdriver] %v", err)
				return nil, err
			}

			log.G(ctx).Infof("[graphdriver] using prior storage driver: %s", name)
			return driver, nil
		}
	}

	// If no prior state was found, continue with automatic selection, and pick
	// the first supported, non-deprecated, storage driver (in order of priorityList).
	for _, name := range priorityList {
		driver, err := getBuiltinDriver(name, config.Root, config.DriverOptions, config.IDMap)
		if err != nil {
			if IsDriverNotSupported(err) {
				continue
			}
			return nil, err
		}
		return driver, nil
	}

	// Check all registered drivers if no priority driver is found
	for name, initFunc := range drivers {
		driver, err := initFunc(filepath.Join(config.Root, name), config.DriverOptions, config.IDMap)
		if err != nil {
			if IsDriverNotSupported(err) {
				continue
			}
			return nil, err
		}
		return driver, nil
	}

	return nil, errors.Errorf("no supported storage driver found")
}

// scanPriorDrivers returns an un-ordered scan of directories of prior storage
// drivers. The 'vfs' storage driver is not taken into account, and ignored.
func scanPriorDrivers(root string) map[string]bool {
	driversMap := make(map[string]bool)

	for driver := range drivers {
		p := filepath.Join(root, driver)
		if _, err := os.Stat(p); err == nil && driver != "vfs" {
			if !isEmptyDir(p) {
				driversMap[driver] = true
			}
		}
	}
	return driversMap
}

// isEmptyDir checks if a directory is empty. It is used to check if prior
// storage-driver directories exist. If an error occurs, it also assumes the
// directory is not empty (which preserves the behavior _before_ this check
// was added)
func isEmptyDir(name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()

	if _, err = f.Readdirnames(1); err == io.EOF {
		return true
	}
	return false
}

// checkRemoved checks if a storage-driver has been deprecated (and removed)
func checkRemoved(name string) error {
	switch name {
	case "aufs", "devicemapper", "overlay":
		return NotSupportedError(fmt.Sprintf("[graphdriver] ERROR: the %s storage-driver has been deprecated and removed; visit https://docs.docker.com/go/storage-driver/ for more information", name))
	}
	return nil
}
