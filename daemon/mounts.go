package daemon

import (
	"strings"

	derr "github.com/docker/docker/errors"
	volumestore "github.com/docker/docker/volume/store"
)

func (daemon *Daemon) prepareMountPoints(container *Container) error {
	for _, config := range container.MountPoints {
		if len(config.Driver) > 0 {
			v, err := daemon.createVolume(config.Name, config.Driver, nil)
			if err != nil {
				return err
			}
			config.Volume = v
		}
	}
	return nil
}

func (daemon *Daemon) removeMountPoints(container *Container, rm bool) error {
	var rmErrors []string
	for _, m := range container.MountPoints {
		if m.Volume == nil {
			continue
		}
		daemon.volumes.Decrement(m.Volume)
		if rm {
			err := daemon.volumes.Remove(m.Volume)
			// ErrVolumeInUse is ignored because having this
			// volume being referenced by other container is
			// not an error, but an implementation detail.
			// This prevents docker from logging "ERROR: Volume in use"
			// where there is another container using the volume.
			if err != nil && !volumestore.IsInUse(err) {
				rmErrors = append(rmErrors, err.Error())
			}
		}
	}
	if len(rmErrors) > 0 {
		return derr.ErrorCodeRemovingVolume.WithArgs(strings.Join(rmErrors, "\n"))
	}
	return nil
}
