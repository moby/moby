package volume

import (
	// TODO return types need to be refactored into pkg
	"github.com/docker/docker/api/types"
)

// Backend is the methods that need to be implemented to provide
// volume specific functionality
type Backend interface {
	Volumes(filter string) ([]*types.Volume, error)
	VolumeInspect(name string) (*types.Volume, error)
	VolumeCreate(name, driverName string,
		opts map[string]string) (*types.Volume, error)
	VolumeRm(name string) error
}
