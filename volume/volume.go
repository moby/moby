package volume

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/stringid"
	"github.com/opencontainers/runc/libcontainer/label"
	"github.com/pkg/errors"
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
	Source      string          // Container host directory
	Destination string          // Inside the container
	RW          bool            // True if writable
	Name        string          // Name set by user
	Driver      string          // Volume driver to use
	Type        mounttypes.Type `json:",omitempty"` // Type of mount to use, see `Type<foo>` definitions
	Volume      Volume          `json:"-"`

	// Note Mode is not used on Windows
	Mode string `json:"Relabel,omitempty"` // Originally field was `Relabel`"

	// Note Propagation is not used on Windows
	Propagation mounttypes.Propagation `json:",omitempty"` // Mount propagation string

	// Specifies if data should be copied from the container before the first mount
	// Use a pointer here so we can tell if the user set this value explicitly
	// This allows us to error out when the user explicitly enabled copy but we can't copy due to the volume being populated
	CopyData bool `json:"-"`
	// ID is the opaque ID used to pass to the volume driver.
	// This should be set by calls to `Mount` and unset by calls to `Unmount`
	ID   string `json:",omitempty"`
	Spec mounttypes.Mount
}

// Setup sets up a mount point by either mounting the volume if it is
// configured, or creating the source directory if supplied.
func (m *MountPoint) Setup(mountLabel string, rootUID, rootGID int) (string, error) {
	if m.Volume != nil {
		if m.ID == "" {
			m.ID = stringid.GenerateNonCryptoID()
		}
		path, err := m.Volume.Mount(m.ID)
		return path, errors.Wrapf(err, "error while mounting volume '%s'", m.Source)
	}
	if len(m.Source) == 0 {
		return "", fmt.Errorf("Unable to setup mount point, neither source nor volume defined")
	}
	// system.MkdirAll() produces an error if m.Source exists and is a file (not a directory),
	if m.Type == mounttypes.TypeBind {
		// idtools.MkdirAllNewAs() produces an error if m.Source exists and is a file (not a directory)
		// also, makes sure that if the directory is created, the correct remapped rootUID/rootGID will own it
		if err := idtools.MkdirAllNewAs(m.Source, 0755, rootUID, rootGID); err != nil {
			if perr, ok := err.(*os.PathError); ok {
				if perr.Err != syscall.ENOTDIR {
					return "", errors.Wrapf(err, "error while creating mount source path '%s'", m.Source)
				}
			}
		}
	}
	if label.RelabelNeeded(m.Mode) {
		if err := label.Relabel(m.Source, mountLabel, label.IsShared(m.Mode)); err != nil {
			return "", errors.Wrapf(err, "error setting label on mount source '%s'", m.Source)
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

// ParseVolumesFrom ensures that the supplied volumes-from is valid.
func ParseVolumesFrom(spec string) (string, string, error) {
	if len(spec) == 0 {
		return "", "", fmt.Errorf("volumes-from specification cannot be an empty string")
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

// ParseMountRaw parses a raw volume spec (e.g. `-v /foo:/bar:shared`) into a
// structured spec. Once the raw spec is parsed it relies on `ParseMountSpec` to
// validate the spec and create a MountPoint
func ParseMountRaw(raw, volumeDriver string) (*MountPoint, error) {
	arr, err := splitRawSpec(convertSlash(raw))
	if err != nil {
		return nil, err
	}

	var spec mounttypes.Mount
	var mode string
	switch len(arr) {
	case 1:
		// Just a destination path in the container
		spec.Target = arr[0]
	case 2:
		if ValidMountMode(arr[1]) {
			// Destination + Mode is not a valid volume - volumes
			// cannot include a mode. eg /foo:rw
			return nil, errInvalidSpec(raw)
		}
		// Host Source Path or Name + Destination
		spec.Source = arr[0]
		spec.Target = arr[1]
	case 3:
		// HostSourcePath+DestinationPath+Mode
		spec.Source = arr[0]
		spec.Target = arr[1]
		mode = arr[2]
	default:
		return nil, errInvalidSpec(raw)
	}

	if !ValidMountMode(mode) {
		return nil, errInvalidMode(mode)
	}

	if filepath.IsAbs(spec.Source) {
		spec.Type = mounttypes.TypeBind
	} else {
		spec.Type = mounttypes.TypeVolume
	}

	spec.ReadOnly = !ReadWrite(mode)

	// cannot assume that if a volume driver is passed in that we should set it
	if volumeDriver != "" && spec.Type == mounttypes.TypeVolume {
		spec.VolumeOptions = &mounttypes.VolumeOptions{
			DriverConfig: &mounttypes.Driver{Name: volumeDriver},
		}
	}

	if copyData, isSet := getCopyMode(mode); isSet {
		if spec.VolumeOptions == nil {
			spec.VolumeOptions = &mounttypes.VolumeOptions{}
		}
		spec.VolumeOptions.NoCopy = !copyData
	}
	if HasPropagation(mode) {
		spec.BindOptions = &mounttypes.BindOptions{
			Propagation: GetPropagation(mode),
		}
	}

	mp, err := ParseMountSpec(spec, platformRawValidationOpts...)
	if mp != nil {
		mp.Mode = mode
	}
	if err != nil {
		err = fmt.Errorf("%v: %v", errInvalidSpec(raw), err)
	}
	return mp, err
}

// ParseMountSpec reads a mount config, validates it, and configures a mountpoint from it.
func ParseMountSpec(cfg mounttypes.Mount, options ...func(*validateOpts)) (*MountPoint, error) {
	if err := validateMountConfig(&cfg, options...); err != nil {
		return nil, err
	}
	mp := &MountPoint{
		RW:          !cfg.ReadOnly,
		Destination: clean(convertSlash(cfg.Target)),
		Type:        cfg.Type,
		Spec:        cfg,
	}

	switch cfg.Type {
	case mounttypes.TypeVolume:
		if cfg.Source == "" {
			mp.Name = stringid.GenerateNonCryptoID()
		} else {
			mp.Name = cfg.Source
		}
		mp.CopyData = DefaultCopyMode

		mp.Driver = DefaultDriverName
		if cfg.VolumeOptions != nil {
			if cfg.VolumeOptions.DriverConfig != nil {
				mp.Driver = cfg.VolumeOptions.DriverConfig.Name
			}
			if cfg.VolumeOptions.NoCopy {
				mp.CopyData = false
			}
		}
	case mounttypes.TypeBind:
		mp.Source = clean(convertSlash(cfg.Source))
		if cfg.BindOptions != nil {
			if len(cfg.BindOptions.Propagation) > 0 {
				mp.Propagation = cfg.BindOptions.Propagation
			}
		}
	}
	return mp, nil
}

func errInvalidMode(mode string) error {
	return fmt.Errorf("invalid mode: %v", mode)
}

func errInvalidSpec(spec string) error {
	return fmt.Errorf("invalid volume specification: '%s'", spec)
}
