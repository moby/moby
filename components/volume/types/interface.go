package types

import (
	// TODO: the interface should only expose types from this package
	"github.com/docker/docker/volume"
)

const (
	// ComponentType is the name identifying this type of component
	ComponentType = "volumes"
)

// VolumeComponent is the public interface for the volume component. It is used by other
// components to access volume management functionality.
type VolumeComponent interface {
	Create(name, driverName, ref string, opts, labels map[string]string) (volume.Volume, error)
	GetWithRef(name, driver, ref string) (volume.Volume, error)
	Dereference(volume.Volume, string)
	Remove(volume.Volume) error
}
