package builders

import (
	"github.com/docker/docker/api/types"
)

// Volume creates a volume with default values.
// Any number of volume function builder can be pass to augment it.
func Volume(builders ...func(volume *types.Volume)) *types.Volume {
	volume := &types.Volume{
		Name:       "volume",
		Driver:     "local",
		Mountpoint: "/data/volume",
		Scope:      "local",
	}

	for _, builder := range builders {
		builder(volume)
	}

	return volume
}

// VolumeLabels sets the volume labels
func VolumeLabels(labels map[string]string) func(volume *types.Volume) {
	return func(volume *types.Volume) {
		volume.Labels = labels
	}
}

// VolumeName sets the volume labels
func VolumeName(name string) func(volume *types.Volume) {
	return func(volume *types.Volume) {
		volume.Name = name
	}
}

// VolumeDriver sets the volume driver
func VolumeDriver(name string) func(volume *types.Volume) {
	return func(volume *types.Volume) {
		volume.Driver = name
	}
}
