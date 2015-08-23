//go:generate pluginrpc-gen -i $GOFILE -o proxy.go -type VolumeDriver -name VolumeDriver

package volumedrivers

import "github.com/docker/docker/volume"

// NewVolumeDriver returns a driver has the given name mapped on the given client.
func NewVolumeDriver(name string, c client) volume.Driver {
	proxy := &volumeDriverProxy{c}
	return &volumeDriverAdapter{name, proxy}
}

// VolumeDriver defines the available functions that volume plugins must implement.
type VolumeDriver interface {
	// Create a volume with the given name
	Create(name string) (err error)
	// Remove the volume with the given name
	Remove(name string) (err error)
	// Get the mountpoint of the given volume
	Path(name string) (mountpoint string, err error)
	// Mount the given volume and return the mountpoint
	Mount(name string) (mountpoint string, err error)
	// Unmount the given volume
	Unmount(name string) (err error)
}
