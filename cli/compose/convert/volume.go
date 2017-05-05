package convert

import (
	"github.com/docker/docker/api/types/mount"
	composetypes "github.com/docker/docker/cli/compose/types"
	"github.com/pkg/errors"
)

type volumes map[string]composetypes.VolumeConfig

// Volumes from compose-file types to engine api types
func Volumes(serviceVolumes []composetypes.ServiceVolumeConfig, stackVolumes volumes, namespace Namespace) ([]mount.Mount, error) {
	var mounts []mount.Mount

	for _, volumeConfig := range serviceVolumes {
		mount, err := convertVolumeToMount(volumeConfig, stackVolumes, namespace)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, mount)
	}
	return mounts, nil
}

func convertVolumeToMount(
	volume composetypes.ServiceVolumeConfig,
	stackVolumes volumes,
	namespace Namespace,
) (mount.Mount, error) {
	result := mount.Mount{
		Type:        mount.Type(volume.Type),
		Source:      volume.Source,
		Target:      volume.Target,
		ReadOnly:    volume.ReadOnly,
		Consistency: mount.Consistency(volume.Consistency),
	}

	// Anonymous volumes
	if volume.Source == "" {
		return result, nil
	}
	if volume.Type == "volume" && volume.Bind != nil {
		return result, errors.New("bind options are incompatible with type volume")
	}
	if volume.Type == "bind" && volume.Volume != nil {
		return result, errors.New("volume options are incompatible with type bind")
	}

	if volume.Bind != nil {
		result.BindOptions = &mount.BindOptions{
			Propagation: mount.Propagation(volume.Bind.Propagation),
		}
	}
	// Binds volumes
	if volume.Type == "bind" {
		return result, nil
	}

	stackVolume, exists := stackVolumes[volume.Source]
	if !exists {
		return result, errors.Errorf("undefined volume %q", volume.Source)
	}

	result.Source = namespace.Scope(volume.Source)
	result.VolumeOptions = &mount.VolumeOptions{}

	if volume.Volume != nil {
		result.VolumeOptions.NoCopy = volume.Volume.NoCopy
	}

	// External named volumes
	if stackVolume.External.External {
		result.Source = stackVolume.External.Name
		return result, nil
	}

	result.VolumeOptions.Labels = AddStackLabel(namespace, stackVolume.Labels)
	if stackVolume.Driver != "" || stackVolume.DriverOpts != nil {
		result.VolumeOptions.DriverConfig = &mount.Driver{
			Name:    stackVolume.Driver,
			Options: stackVolume.DriverOpts,
		}
	}

	// Named volumes
	return result, nil
}
