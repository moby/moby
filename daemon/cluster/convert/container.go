package convert

import (
	"fmt"
	"strings"

	mounttypes "github.com/docker/docker/api/types/mount"
	types "github.com/docker/docker/api/types/swarm"
	swarmapi "github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/protobuf/ptypes"
)

func containerSpecFromGRPC(c *swarmapi.ContainerSpec) types.ContainerSpec {
	containerSpec := types.ContainerSpec{
		Image:   c.Image,
		Labels:  c.Labels,
		Command: c.Command,
		Args:    c.Args,
		Env:     c.Env,
		Dir:     c.Dir,
		User:    c.User,
		Groups:  c.Groups,
	}

	// Mounts
	for _, m := range c.Mounts {
		mount := mounttypes.Mount{
			Target:   m.Target,
			Source:   m.Source,
			Type:     mounttypes.Type(strings.ToLower(swarmapi.Mount_MountType_name[int32(m.Type)])),
			ReadOnly: m.ReadOnly,
		}

		if m.BindOptions != nil {
			mount.BindOptions = &mounttypes.BindOptions{
				Propagation: mounttypes.Propagation(strings.ToLower(swarmapi.Mount_BindOptions_MountPropagation_name[int32(m.BindOptions.Propagation)])),
			}
		}

		if m.VolumeOptions != nil {
			mount.VolumeOptions = &mounttypes.VolumeOptions{
				NoCopy: m.VolumeOptions.NoCopy,
				Labels: m.VolumeOptions.Labels,
			}
			if m.VolumeOptions.DriverConfig != nil {
				mount.VolumeOptions.DriverConfig = &mounttypes.Driver{
					Name:    m.VolumeOptions.DriverConfig.Name,
					Options: m.VolumeOptions.DriverConfig.Options,
				}
			}
		}
		containerSpec.Mounts = append(containerSpec.Mounts, mount)
	}

	if c.StopGracePeriod != nil {
		grace, _ := ptypes.Duration(c.StopGracePeriod)
		containerSpec.StopGracePeriod = &grace
	}
	return containerSpec
}

func containerToGRPC(c types.ContainerSpec) (*swarmapi.ContainerSpec, error) {
	containerSpec := &swarmapi.ContainerSpec{
		Image:   c.Image,
		Labels:  c.Labels,
		Command: c.Command,
		Args:    c.Args,
		Env:     c.Env,
		Dir:     c.Dir,
		User:    c.User,
		Groups:  c.Groups,
	}

	if c.StopGracePeriod != nil {
		containerSpec.StopGracePeriod = ptypes.DurationProto(*c.StopGracePeriod)
	}

	// Mounts
	for _, m := range c.Mounts {
		mount := swarmapi.Mount{
			Target:   m.Target,
			Source:   m.Source,
			ReadOnly: m.ReadOnly,
		}

		if mountType, ok := swarmapi.Mount_MountType_value[strings.ToUpper(string(m.Type))]; ok {
			mount.Type = swarmapi.Mount_MountType(mountType)
		} else if string(m.Type) != "" {
			return nil, fmt.Errorf("invalid MountType: %q", m.Type)
		}

		if m.BindOptions != nil {
			if mountPropagation, ok := swarmapi.Mount_BindOptions_MountPropagation_value[strings.ToUpper(string(m.BindOptions.Propagation))]; ok {
				mount.BindOptions = &swarmapi.Mount_BindOptions{Propagation: swarmapi.Mount_BindOptions_MountPropagation(mountPropagation)}
			} else if string(m.BindOptions.Propagation) != "" {
				return nil, fmt.Errorf("invalid MountPropagation: %q", m.BindOptions.Propagation)

			}

		}

		if m.VolumeOptions != nil {
			mount.VolumeOptions = &swarmapi.Mount_VolumeOptions{
				NoCopy: m.VolumeOptions.NoCopy,
				Labels: m.VolumeOptions.Labels,
			}
			if m.VolumeOptions.DriverConfig != nil {
				mount.VolumeOptions.DriverConfig = &swarmapi.Driver{
					Name:    m.VolumeOptions.DriverConfig.Name,
					Options: m.VolumeOptions.DriverConfig.Options,
				}
			}
		}

		containerSpec.Mounts = append(containerSpec.Mounts, mount)
	}

	return containerSpec, nil
}
