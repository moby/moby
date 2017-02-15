package convert

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/mount"
	composetypes "github.com/docker/docker/cli/compose/types"
)

type volumes map[string]composetypes.VolumeConfig

// Volumes from compose-file types to engine api types
func Volumes(serviceVolumes []string, stackVolumes volumes, namespace Namespace) ([]mount.Mount, error) {
	var mounts []mount.Mount

	for _, volumeSpec := range serviceVolumes {
		mount, err := convertVolumeToMount(volumeSpec, stackVolumes, namespace)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, mount)
	}
	return mounts, nil
}

func convertVolumeToMount(volumeSpec string, stackVolumes volumes, namespace Namespace) (mount.Mount, error) {
	var source, target string
	var mode []string

	// TODO: split Windows path mappings properly
	parts := strings.SplitN(volumeSpec, ":", 3)

	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return mount.Mount{}, fmt.Errorf("invalid volume: %s", volumeSpec)
		}
	}

	switch len(parts) {
	case 3:
		source = parts[0]
		target = parts[1]
		mode = strings.Split(parts[2], ",")
	case 2:
		source = parts[0]
		target = parts[1]
	case 1:
		target = parts[0]
	}

	if source == "" {
		// Anonymous volume
		return mount.Mount{
			Type:   mount.TypeVolume,
			Target: target,
		}, nil
	}

	// TODO: catch Windows paths here
	if strings.HasPrefix(source, "/") {
		return mount.Mount{
			Type:        mount.TypeBind,
			Source:      source,
			Target:      target,
			ReadOnly:    isReadOnly(mode),
			BindOptions: getBindOptions(mode),
		}, nil
	}

	stackVolume, exists := stackVolumes[source]
	if !exists {
		return mount.Mount{}, fmt.Errorf("undefined volume: %s", source)
	}

	var volumeOptions *mount.VolumeOptions
	if stackVolume.External.Name != "" {
		volumeOptions = &mount.VolumeOptions{
			NoCopy: isNoCopy(mode),
		}
		source = stackVolume.External.Name
	} else {
		volumeOptions = &mount.VolumeOptions{
			Labels: AddStackLabel(namespace, stackVolume.Labels),
			NoCopy: isNoCopy(mode),
		}

		if stackVolume.Driver != "" {
			volumeOptions.DriverConfig = &mount.Driver{
				Name:    stackVolume.Driver,
				Options: stackVolume.DriverOpts,
			}
		}
		source = namespace.Scope(source)
	}
	return mount.Mount{
		Type:          mount.TypeVolume,
		Source:        source,
		Target:        target,
		ReadOnly:      isReadOnly(mode),
		VolumeOptions: volumeOptions,
	}, nil
}

func modeHas(mode []string, field string) bool {
	for _, item := range mode {
		if item == field {
			return true
		}
	}
	return false
}

func isReadOnly(mode []string) bool {
	return modeHas(mode, "ro")
}

func isNoCopy(mode []string) bool {
	return modeHas(mode, "nocopy")
}

func getBindOptions(mode []string) *mount.BindOptions {
	for _, item := range mode {
		for _, propagation := range mount.Propagations {
			if mount.Propagation(item) == propagation {
				return &mount.BindOptions{Propagation: mount.Propagation(item)}
			}
		}
	}
	return nil
}
