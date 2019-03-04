package convert

import (
	"github.com/docker/docker/api/types/mount"
	composetypes "github.com/docker/stacks/pkg/compose/types"
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

func createMountFromVolume(volume composetypes.ServiceVolumeConfig) mount.Mount {
	return mount.Mount{
		Type:        mount.Type(volume.Type),
		Target:      volume.Target,
		ReadOnly:    volume.ReadOnly,
		Source:      volume.Source,
		Consistency: mount.Consistency(volume.Consistency),
	}
}

func handleVolumeToMount(
	volume composetypes.ServiceVolumeConfig,
	stackVolumes volumes,
	namespace Namespace,
) (mount.Mount, error) {
	result := createMountFromVolume(volume)

	if volume.Tmpfs != nil {
		return mount.Mount{}, errors.New("tmpfs options are incompatible with type volume")
	}
	if volume.Bind != nil {
		return mount.Mount{}, errors.New("bind options are incompatible with type volume")
	}
	// Anonymous volumes
	if volume.Source == "" {
		return result, nil
	}

	stackVolume, exists := stackVolumes[volume.Source]
	if !exists {
		return mount.Mount{}, errors.Errorf("undefined volume %q", volume.Source)
	}

	result.Source = namespace.Scope(volume.Source)
	result.VolumeOptions = &mount.VolumeOptions{}

	if volume.Volume != nil {
		result.VolumeOptions.NoCopy = volume.Volume.NoCopy
	}

	if stackVolume.Name != "" {
		result.Source = stackVolume.Name
	}

	// External named volumes
	if stackVolume.External.External {
		return result, nil
	}

	result.VolumeOptions.Labels = AddStackLabel(namespace, stackVolume.Labels)
	if stackVolume.Driver != "" || stackVolume.DriverOpts != nil {
		result.VolumeOptions.DriverConfig = &mount.Driver{
			Name:    stackVolume.Driver,
			Options: stackVolume.DriverOpts,
		}
	}

	return result, nil
}

func handleBindToMount(volume composetypes.ServiceVolumeConfig) (mount.Mount, error) {
	result := createMountFromVolume(volume)

	if volume.Source == "" {
		return mount.Mount{}, errors.New("invalid bind source, source cannot be empty")
	}
	if volume.Volume != nil {
		return mount.Mount{}, errors.New("volume options are incompatible with type bind")
	}
	if volume.Tmpfs != nil {
		return mount.Mount{}, errors.New("tmpfs options are incompatible with type bind")
	}
	if volume.Bind != nil {
		result.BindOptions = &mount.BindOptions{
			Propagation: mount.Propagation(volume.Bind.Propagation),
		}
	}
	return result, nil
}

func handleTmpfsToMount(volume composetypes.ServiceVolumeConfig) (mount.Mount, error) {
	result := createMountFromVolume(volume)

	if volume.Source != "" {
		return mount.Mount{}, errors.New("invalid tmpfs source, source must be empty")
	}
	if volume.Bind != nil {
		return mount.Mount{}, errors.New("bind options are incompatible with type tmpfs")
	}
	if volume.Volume != nil {
		return mount.Mount{}, errors.New("volume options are incompatible with type tmpfs")
	}
	if volume.Tmpfs != nil {
		result.TmpfsOptions = &mount.TmpfsOptions{
			SizeBytes: volume.Tmpfs.Size,
		}
	}
	return result, nil
}

func handleNpipeToMount(volume composetypes.ServiceVolumeConfig) (mount.Mount, error) {
	result := createMountFromVolume(volume)

	if volume.Source == "" {
		return mount.Mount{}, errors.New("invalid npipe source, source cannot be empty")
	}
	if volume.Volume != nil {
		return mount.Mount{}, errors.New("volume options are incompatible with type npipe")
	}
	if volume.Tmpfs != nil {
		return mount.Mount{}, errors.New("tmpfs options are incompatible with type npipe")
	}
	if volume.Bind != nil {
		result.BindOptions = &mount.BindOptions{
			Propagation: mount.Propagation(volume.Bind.Propagation),
		}
	}
	return result, nil
}

func convertVolumeToMount(
	volume composetypes.ServiceVolumeConfig,
	stackVolumes volumes,
	namespace Namespace,
) (mount.Mount, error) {

	switch volume.Type {
	case "volume", "":
		return handleVolumeToMount(volume, stackVolumes, namespace)
	case "bind":
		return handleBindToMount(volume)
	case "tmpfs":
		return handleTmpfsToMount(volume)
	case "npipe":
		return handleNpipeToMount(volume)
	}
	return mount.Mount{}, errors.New("volume type must be volume, bind, tmpfs or npipe")
}
