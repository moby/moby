package convert

import (
	"fmt"

	types "github.com/docker/engine-api/types/swarm"
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
	}

	// Mounts
	for _, m := range c.Mounts {
		mount := types.Mount{
			Target:        m.Target,
			Source:        m.Source,
			Type:          types.MountType(swarmapi.Mount_MountType_name[int32(m.Type)]),
			Populate:      m.Populate,
			Propagation:   types.MountPropagation(swarmapi.Mount_MountPropagation_name[int32(m.Propagation)]),
			MCSAccessMode: types.MountMCSAaccessMode(swarmapi.Mount_MountMCSAccessMode_name[int32(m.Mcsaccessmode)]),
			Writable:      m.Writable,
			// TODO: template
		}

		if m.Template != nil {
			mount.Template.Name = m.Template.Annotations.Name
			mount.Template.Labels = m.Template.Annotations.Labels
			mount.Template.DriverConfig = types.Driver{
				Name:    m.Template.DriverConfig.Name,
				Options: m.Template.DriverConfig.Options,
			}
		}
		containerSpec.Mounts = append(containerSpec.Mounts, mount)
	}

	grace, _ := ptypes.Duration(&c.StopGracePeriod)
	containerSpec.StopGracePeriod = &grace
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
	}

	if c.StopGracePeriod != nil {
		containerSpec.StopGracePeriod = *ptypes.DurationProto(*c.StopGracePeriod)
	}

	// Mounts
	for _, m := range c.Mounts {
		mount := swarmapi.Mount{
			Target:   m.Target,
			Source:   m.Source,
			Populate: m.Populate,
			Writable: m.Writable,
		}

		if mountType, ok := swarmapi.Mount_MountType_value[string(m.Type)]; ok {
			mount.Type = swarmapi.Mount_MountType(mountType)
		} else if string(m.Type) != "" {
			return nil, fmt.Errorf("invalid MountType: %q", m.Type)
		}

		if mountPropagation, ok := swarmapi.Mount_MountPropagation_value[string(m.Propagation)]; ok {
			mount.Propagation = swarmapi.Mount_MountPropagation(mountPropagation)
		} else if string(m.Propagation) != "" {
			return nil, fmt.Errorf("invalid MountPropagation: %q", m.Propagation)

		}

		if mountMCSAccessMode, ok := swarmapi.Mount_MountMCSAccessMode_value[string(m.MCSAccessMode)]; ok {
			mount.Mcsaccessmode = swarmapi.Mount_MountMCSAccessMode(mountMCSAccessMode)
		} else if string(m.MCSAccessMode) != "" {
			return nil, fmt.Errorf("invalid MountMCSAccessMode: %q", m.MCSAccessMode)

		}

		if m.Template != nil {
			mount.Template.Annotations.Name = m.Template.Name
			mount.Template.Annotations.Labels = m.Template.Labels
			if m.Template.DriverConfig.Name != "" || m.Template.DriverConfig.Options != nil {
				mount.Template.DriverConfig = &swarmapi.Driver{
					Name:    m.Template.DriverConfig.Name,
					Options: m.Template.DriverConfig.Options,
				}
			}
		}

		containerSpec.Mounts = append(containerSpec.Mounts, &mount)
	}

	return containerSpec, nil
}
