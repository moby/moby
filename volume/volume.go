package volume

import (
	"os"
	"runtime"
	"strings"

	"github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/system"
)

// DefaultDriverName is the driver name used for the driver
// implemented in the local package.
const DefaultDriverName string = "local"

// Driver is for creating and removing volumes.
type Driver interface {
	// Name returns the name of the volume driver.
	Name() string
	// Create makes a new volume with the given id.
	Create(name string, opts map[string]string) (Volume, error)
	// Remove deletes the volume.
	Remove(Volume) error
}

// Volume is a place to store data. It is backed by a specific driver, and can be mounted.
type Volume interface {
	// Name returns the name of the volume
	Name() string
	// DriverName returns the name of the driver which owns this volume.
	DriverName() string
	// Path returns the absolute path to the volume.
	Path() string
	// Mount mounts the volume and returns the absolute path to
	// where it can be consumed.
	Mount() (string, error)
	// Unmount unmounts the volume when it is no longer in use.
	Unmount() error
}

// MountPoint is the intersection point between a volume and a container. It
// specifies which volume is to be used and where inside a container it should
// be mounted.
type MountPoint struct {
	Source      string // Container host directory
	Destination string // Inside the container
	RW          bool   // True if writable
	Name        string // Name set by user
	Driver      string // Volume driver to use
	Volume      Volume `json:"-"`

	// Note Mode is not used on Windows
	Mode string `json:"Relabel"` // Originally field was `Relabel`"

	// Note Propagation is not used on Windows
	Propagation string // Mount propagation string
}

// Setup sets up a mount point by either mounting the volume if it is
// configured, or creating the source directory if supplied.
func (m *MountPoint) Setup() (string, error) {
	if m.Volume != nil {
		return m.Volume.Mount()
	}
	if len(m.Source) > 0 {
		if _, err := os.Stat(m.Source); err != nil {
			if !os.IsNotExist(err) {
				return "", err
			}
			if runtime.GOOS != "windows" { // Windows does not have deprecation issues here
				logrus.Warnf("Auto-creating non-existent volume host path %s, this is deprecated and will be removed soon", m.Source)
				if err := system.MkdirAll(m.Source, 0755); err != nil {
					return "", err
				}
			}
		}
		return m.Source, nil
	}
	return "", derr.ErrorCodeMountSetup
}

// Path returns the path of a volume in a mount point.
func (m *MountPoint) Path() string {
	if m.Volume != nil {
		return m.Volume.Path()
	}
	return m.Source
}

// ParseVolumesFrom ensure that the supplied volumes-from is valid.
func ParseVolumesFrom(spec string) (string, string, error) {
	if len(spec) == 0 {
		return "", "", derr.ErrorCodeVolumeFromBlank.WithArgs(spec)
	}

	specParts := strings.SplitN(spec, ":", 2)
	id := specParts[0]
	mode := "rw"

	if len(specParts) == 2 {
		mode = specParts[1]
		if !ValidMountMode(mode) {
			return "", "", derr.ErrorCodeVolumeInvalidMode.WithArgs(mode)
		}
		// For now don't allow propagation properties while importing
		// volumes from data container. These volumes will inherit
		// the same propagation property as of the original volume
		// in data container. This probably can be relaxed in future.
		if HasPropagation(mode) {
			return "", "", derr.ErrorCodeVolumeInvalidMode.WithArgs(mode)
		}
	}
	return id, mode, nil
}
