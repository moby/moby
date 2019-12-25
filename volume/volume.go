package volume // import "github.com/docker/docker/volume"

import (
	"time"
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
	// Create makes a new volume with the given name.
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
	// CreatedAt returns Volume Creation time
	CreatedAt() (time.Time, error)
	// Status returns low-level status information about a volume
	Status() map[string]interface{}
}

// DetailedVolume wraps a Volume with user-defined labels, options, and cluster scope (e.g., `local` or `global`)
type DetailedVolume interface {
	Labels() map[string]string
	Options() map[string]string
	Scope() string
	Volume
}
