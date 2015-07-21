package volume

const DefaultDriverName = "local"

type Driver interface {
	// Name returns the name of the volume driver.
	Name() string
	// Create makes a new volume with the given name and id.
	Create(string, string) (Volume, error)
	// Remove deletes the volume.
	Remove(Volume) error
}

type Volume interface {
	// Name returns the name of the volume
	Name() string
	// Id returns the unique volume UID
	Id() string
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

// read-write modes
var rwModes = map[string]bool{
	"rw":   true,
	"rw,Z": true,
	"rw,z": true,
	"z,rw": true,
	"Z,rw": true,
	"Z":    true,
	"z":    true,
}

// read-only modes
var roModes = map[string]bool{
	"ro":   true,
	"ro,Z": true,
	"ro,z": true,
	"z,ro": true,
	"Z,ro": true,
}

// ValidateMountMode will make sure the mount mode is valid.
// returns if it's a valid mount mode and if it's read-write or not.
func ValidateMountMode(mode string) (bool, bool) {
	return roModes[mode] || rwModes[mode], rwModes[mode]
}

// ReadOnly tells you if a mode string is a valid read-only mode or not.
func ReadWrite(mode string) bool {
	return rwModes[mode]
}
