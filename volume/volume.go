package volume

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

// ValidMountMode will make sure the mount mode is valid.
// returns if it's a valid mount mode or not.
func ValidMountMode(mode string) bool {
	return roModes[mode] || rwModes[mode]
}

// ReadWrite tells you if a mode string is a valid read-write mode or not.
func ReadWrite(mode string) bool {
	return rwModes[mode]
}
