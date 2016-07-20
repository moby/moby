package volume

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/system"
	"github.com/opencontainers/runc/libcontainer/label"
)

// DefaultDriverName is the driver name used for the driver
// implemented in the local package.
const DefaultDriverName = "local"

// Scopes define if a volume has is cluster-wide (global) or local only.
// Scopes are returned by the volume driver when it is queried for capabilities and then set on a volume
const (
	LocalScope  = "local"
	GlobalScope = "global"
)

// Driver is for creating and removing volumes.
type Driver interface {
	// Name returns the name of the volume driver.
	Name() string
	// Create makes a new volume with the given id.
	Create(name string, opts map[string]string) (Volume, error)
	// Remove deletes the volume.
	Remove(vol Volume) (err error)
	// List lists all the volumes the driver has
	List() ([]Volume, error)
	// Get retrieves the volume with the requested name
	Get(name string) (Volume, error)
	// Scope returns the scope of the driver (e.g. `global` or `local`).
	// Scope determines how the driver is handled at a cluster level
	Scope() string
}

// Capability defines a set of capabilities that a driver is able to handle.
type Capability struct {
	// Scope is the scope of the driver, `global` or `local`
	// A `global` scope indicates that the driver manages volumes across the cluster
	// A `local` scope indicates that the driver only manages volumes resources local to the host
	// Scope is declared by the driver
	Scope string
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
	Mount(id string) (string, error)
	// Unmount unmounts the volume when it is no longer in use.
	Unmount(id string) error
	// Status returns low-level status information about a volume
	Status() map[string]interface{}
}

// LabeledVolume wraps a Volume with user-defined labels
type LabeledVolume interface {
	Labels() map[string]string
	Volume
}

// ScopedVolume wraps a volume with a cluster scope (e.g., `local` or `global`)
type ScopedVolume interface {
	Scope() string
	Volume
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
	Named       bool   // specifies if the mountpoint was specified by name

	// Specifies if data should be copied from the container before the first mount
	// Use a pointer here so we can tell if the user set this value explicitly
	// This allows us to error out when the user explicitly enabled copy but we can't copy due to the volume being populated
	CopyData bool `json:"-"`
	// ID is the opaque ID used to pass to the volume driver.
	// This should be set by calls to `Mount` and unset by calls to `Unmount`
	ID string
}

// Setup sets up a mount point by either mounting the volume if it is
// configured, or creating the source directory if supplied.
func (m *MountPoint) Setup(mountLabel string) (string, error) {
	if m.Volume != nil {
		if m.ID == "" {
			m.ID = stringid.GenerateNonCryptoID()
		}
		return m.Volume.Mount(m.ID)
	}
	if len(m.Source) == 0 {
		return "", fmt.Errorf("Unable to setup mount point, neither source nor volume defined")
	}
	// system.MkdirAll() produces an error if m.Source exists and is a file (not a directory),
	if err := system.MkdirAll(m.Source, 0755); err != nil {
		if perr, ok := err.(*os.PathError); ok {
			if perr.Err != syscall.ENOTDIR {
				return "", err
			}
		}
	}
	if label.RelabelNeeded(m.Mode) {
		if err := label.Relabel(m.Source, mountLabel, label.IsShared(m.Mode)); err != nil {
			return "", err
		}
	}
	return m.Source, nil
}

// Path returns the path of a volume in a mount point.
func (m *MountPoint) Path() string {
	if m.Volume != nil {
		return m.Volume.Path()
	}
	return m.Source
}

// Type returns the type of mount point
func (m *MountPoint) Type() string {
	if m.Name != "" {
		return "volume"
	}
	if m.Source != "" {
		return "bind"
	}
	return "ephemeral"
}

// ParseVolumesFrom ensures that the supplied volumes-from is valid.
func ParseVolumesFrom(spec string) (string, string, error) {
	if len(spec) == 0 {
		return "", "", fmt.Errorf("malformed volumes-from specification: %s", spec)
	}

	specParts := strings.SplitN(spec, ":", 2)
	id := specParts[0]
	mode := "rw"

	if len(specParts) == 2 {
		mode = specParts[1]
		if !ValidMountMode(mode) {
			return "", "", errInvalidMode(mode)
		}
		// For now don't allow propagation properties while importing
		// volumes from data container. These volumes will inherit
		// the same propagation property as of the original volume
		// in data container. This probably can be relaxed in future.
		if HasPropagation(mode) {
			return "", "", errInvalidMode(mode)
		}
		// Do not allow copy modes on volumes-from
		if _, isSet := getCopyMode(mode); isSet {
			return "", "", errInvalidMode(mode)
		}
	}
	return id, mode, nil
}

func errInvalidMode(mode string) error {
	return fmt.Errorf("invalid mode: %v", mode)
}

func errInvalidSpec(spec string) error {
	return fmt.Errorf("Invalid volume specification: '%s'", spec)
}
