package graphdriver

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/idtools"
)

type validateFunc func(root string) error
type initFunc func(root string, options []string, uidMaps, gidMaps []idtools.IDMap) (Driver, error)

// Bootstrap is an interface that includes functions to
// validate and initialize a graph driver.
type Bootstrap interface {
	ValidateSupport(root string) error
	Init(root string, options []string, uidMaps, gidMaps []idtools.IDMap) (Driver, error)
}

// FSBootstrap is a Bootstrap struct that ensures
// the filesystem hierarchy is generated correctly
// for the engine's normal use.
type FSBootstrap struct {
	validateSupport validateFunc
	driverInit      initFunc
}

// NewFSBootstrap creates a new FSBootstrap with a validate and an init function.
func NewFSBootstrap(validate validateFunc, init initFunc) Bootstrap {
	return FSBootstrap{
		validateSupport: validate,
		driverInit:      init,
	}
}

// NewAlwaysValidFSBootstrap creates a new FSBootstrap object
// which support validation is always true, like vfs on linux.
func NewAlwaysValidFSBootstrap(init initFunc) Bootstrap {
	return FSBootstrap{
		validateSupport: alwaysValidDriver,
		driverInit:      init,
	}
}

// ValidateSupport returns an error if the graph driver
// is not supported in the host.
func (b FSBootstrap) ValidateSupport(root string) error {
	return b.validateSupport(root)
}

// Init generates the root directory hierarchy and initializes the graph driver.
func (b FSBootstrap) Init(graphRoot string, options []string, uidMaps, gidMaps []idtools.IDMap) (Driver, error) {
	if err := InitRootFilesystem(filepath.Dir(graphRoot), options, uidMaps, gidMaps); err != nil {
		return nil, err
	}

	return b.driverInit(graphRoot, options, uidMaps, gidMaps)
}

// InitRootFilesystem generates the correct root hierarchy for the engine.
func InitRootFilesystem(root string, options []string, uidMaps, gidMaps []idtools.IDMap) error {
	rootUID, rootGID, err := idtools.GetRootUIDGID(uidMaps, gidMaps)
	if err != nil {
		return err
	}

	if err = setupDaemonRoot(root, rootUID, rootGID); err != nil {
		return err
	}

	daemonRepo := filepath.Join(root, "containers")
	if err := idtools.MkdirAllAs(daemonRepo, 0700, rootUID, rootGID); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func alwaysValidDriver(root string) error {
	return nil
}
