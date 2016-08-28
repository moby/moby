// +build windows

package daemon

import (
	"sort"

	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/container"
	"github.com/docker/docker/volume"
	"github.com/pkg/errors"
)

// setupMounts configures the mount points for a container by appending each
// of the configured mounts on the container to the OCI mount structure
// which will ultimately be passed into the oci runtime during container creation.
// It also ensures each of the mounts are lexicographically sorted.

// BUGBUG TODO Windows containerd. This would be much better if it returned
// an array of runtime spec mounts, not container mounts. Then no need to
// do multiple transitions.

func (daemon *Daemon) setupMounts(c *container.Container) ([]container.Mount, error) {
	var mnts []container.Mount
	foundIntrospection := false
	for _, mount := range c.MountPoints { // type is volume.MountPoint
		// TODO(AkihiroSuda): dedupe (volumes_unix.go)
		if mount.Type == mounttypes.TypeIntrospection {
			if !daemon.HasExperimental() {
				return nil, errors.New("introspection mount is only supported in experimental mode")
			}
			if foundIntrospection {
				return nil, errors.Errorf("too many introspection mounts: %+v", mount)
			}
			if mount.RW {
				return nil, errors.Errorf("introspection mount must be read-only: %+v", mount)
			}
			opts := &introspectionOptions{scopes: mount.Spec.IntrospectionOptions.Scopes}
			if err := daemon.updateIntrospection(c, opts); err != nil {
				return nil, err
			}
			mnt := container.Mount{
				Source:      c.IntrospectionDir(),
				Destination: mount.Destination,
				Writable:    false,
			}
			mnts = append(mnts, mnt)
			foundIntrospection = true
			continue
		}

		if err := daemon.lazyInitializeVolume(c.ID, mount); err != nil {
			return nil, err
		}
		s, err := mount.Setup(c.MountLabel, 0, 0)
		if err != nil {
			return nil, err
		}

		mnts = append(mnts, container.Mount{
			Source:      s,
			Destination: mount.Destination,
			Writable:    mount.RW,
		})
	}

	sort.Sort(mounts(mnts))
	return mnts, nil
}

// setBindModeIfNull is platform specific processing which is a no-op on
// Windows.
func setBindModeIfNull(bind *volume.MountPoint) {
	return
}
