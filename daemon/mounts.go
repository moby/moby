package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/log"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/container"
	ctrd "github.com/docker/docker/daemon/containerd"
	volumesservice "github.com/docker/docker/volume/service"
)

func (daemon *Daemon) prepareMountPoints(container *container.Container) error {
	alive := container.IsRunning()
	for _, config := range container.MountPoints {
		if err := daemon.lazyInitializeVolume(container.ID, config); err != nil {
			return err
		}
		if config.Volume == nil {
			// FIXME(thaJeztah): should we check for config.Type here as well? (i.e., skip bind-mounts etc)
			continue
		}
		if alive {
			log.G(context.TODO()).WithFields(log.Fields{
				"container": container.ID,
				"volume":    config.Volume.Name(),
			}).Debug("Live-restoring volume for alive container")
			if err := config.LiveRestore(context.TODO()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (daemon *Daemon) removeMountPoints(container *container.Container, rm bool) error {
	var rmErrors []string
	ctx := context.TODO()
	for _, m := range container.MountPoints {
		if m.Type == mounttypes.TypeVolume {
			if m.Volume == nil {
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

		if m.Type == mounttypes.TypeImage {
			layerName := fmt.Sprintf("%s-%s", container.ID, m.Spec.Source)
			if daemon.UsesSnapshotter() {
				if ctrdImgSvc, ok := daemon.imageService.(*ctrd.ImageService); ok {
					ctrdImgSvc.Mounter().Unmount(m.Source)
					continue
				}

				// This should not happen because of the snapshotter check
				rmErrors = append(rmErrors, fmt.Errorf("Can't get the containerd service").Error())
			} else {
				if accessor, ok := daemon.imageService.(layerAccessor); ok {
					layer, err := accessor.GetLayerByID(layerName)
					if err != nil {
						rmErrors = append(rmErrors, err.Error())
						continue
					}
					err = daemon.imageService.ReleaseLayer(layer)
					if err != nil {
						rmErrors = append(rmErrors, err.Error())
						continue
					}
					err = layer.Unmount()
					if err != nil {
						rmErrors = append(rmErrors, err.Error())
						continue
					}
				}

				// This should not happen because of the snapshotter check
				rmErrors = append(rmErrors, fmt.Errorf("Can't get layer by id").Error())
			}
		}
	}

	if len(rmErrors) > 0 {
		return fmt.Errorf("Error removing volumes:\n%v", strings.Join(rmErrors, "\n"))
	}
	return nil
}
