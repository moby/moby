package daemon // import "github.com/moby/moby/daemon"

import (
	"context"
	"fmt"
	"strings"

	mounttypes "github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/container"
	volumesservice "github.com/moby/moby/volume/service"
)

func (daemon *Daemon) prepareMountPoints(container *container.Container) error {
	for _, config := range container.MountPoints {
		if err := daemon.lazyInitializeVolume(container.ID, config); err != nil {
			return err
		}
	}
	return nil
}

func (daemon *Daemon) removeMountPoints(container *container.Container, rm bool) error {
	var rmErrors []string
	ctx := context.TODO()
	for _, m := range container.MountPoints {
		if m.Type != mounttypes.TypeVolume || m.Volume == nil {
			continue
		}
		daemon.volumes.Release(ctx, m.Volume.Name(), container.ID)
		if !rm {
			continue
		}

		// Do not remove named mountpoints
		// these are mountpoints specified like `docker run -v <name>:/foo`
		if m.Spec.Source != "" {
			continue
		}

		err := daemon.volumes.Remove(ctx, m.Volume.Name())
		// Ignore volume in use errors because having this
		// volume being referenced by other container is
		// not an error, but an implementation detail.
		// This prevents docker from logging "ERROR: Volume in use"
		// where there is another container using the volume.
		if err != nil && !volumesservice.IsInUse(err) {
			rmErrors = append(rmErrors, err.Error())
		}
	}

	if len(rmErrors) > 0 {
		return fmt.Errorf("Error removing volumes:\n%v", strings.Join(rmErrors, "\n"))
	}
	return nil
}
