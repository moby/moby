//go:build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/events"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/container"
	"github.com/docker/docker/internal/cleanups"
	"github.com/docker/docker/internal/compatcontext"
	volumemounts "github.com/docker/docker/volume/mounts"
	"github.com/pkg/errors"
)

// setupMounts iterates through each of the mount points for a container and
// calls Setup() on each. It also looks to see if is a network mount such as
// /etc/resolv.conf, and if it is not, appends it to the array of mounts.
//
// The cleanup function should be called as soon as the container has been
// started.
func (daemon *Daemon) setupMounts(ctx context.Context, c *container.Container) ([]container.Mount, func(context.Context) error, error) {
	var mounts []container.Mount
	// TODO: tmpfs mounts should be part of Mountpoints
	tmpfsMounts := make(map[string]bool)
	tmpfsMountInfo, err := c.TmpfsMounts()
	if err != nil {
		return nil, nil, err
	}
	for _, m := range tmpfsMountInfo {
		tmpfsMounts[m.Destination] = true
	}

	cleanups := cleanups.Composite{}
	defer func() {
		if err := cleanups.Call(compatcontext.WithoutCancel(ctx)); err != nil {
			log.G(ctx).WithError(err).Warn("failed to cleanup temporary mounts created by MountPoint.Setup")
		}
	}()

	for _, m := range c.MountPoints {
		if tmpfsMounts[m.Destination] {
			continue
		}
		if err := daemon.lazyInitializeVolume(c.ID, m); err != nil {
			return nil, nil, err
		}
		// If the daemon is being shutdown, we should not let a container start if it is trying to
		// mount the socket the daemon is listening on. During daemon shutdown, the socket
		// (/var/run/docker.sock by default) doesn't exist anymore causing the call to m.Setup to
		// create at directory instead. This in turn will prevent the daemon to restart.
		checkfunc := func(m *volumemounts.MountPoint) error {
			if _, exist := daemon.hosts[m.Source]; exist && daemon.IsShuttingDown() {
				return fmt.Errorf("Could not mount %q to container while the daemon is shutting down", m.Source)
			}
			return nil
		}

		path, clean, err := m.Setup(ctx, c.MountLabel, daemon.idMapping.RootPair(), checkfunc)
		if err != nil {
			return nil, nil, err
		}
		cleanups.Add(clean)

		if !c.TrySetNetworkMount(m.Destination, path) {
			mnt := container.Mount{
				Source:      path,
				Destination: m.Destination,
				Writable:    m.RW,
				Propagation: string(m.Propagation),
			}
			if m.Spec.Type == mounttypes.TypeBind && m.Spec.BindOptions != nil {
				if !m.Spec.ReadOnly && m.Spec.BindOptions.ReadOnlyNonRecursive {
					return nil, nil, errors.New("mount options conflict: !ReadOnly && BindOptions.ReadOnlyNonRecursive")
				}
				if !m.Spec.ReadOnly && m.Spec.BindOptions.ReadOnlyForceRecursive {
					return nil, nil, errors.New("mount options conflict: !ReadOnly && BindOptions.ReadOnlyForceRecursive")
				}
				if m.Spec.BindOptions.ReadOnlyNonRecursive && m.Spec.BindOptions.ReadOnlyForceRecursive {
					return nil, nil, errors.New("mount options conflict: ReadOnlyNonRecursive && BindOptions.ReadOnlyForceRecursive")
				}
				mnt.NonRecursive = m.Spec.BindOptions.NonRecursive
				mnt.ReadOnlyNonRecursive = m.Spec.BindOptions.ReadOnlyNonRecursive
				mnt.ReadOnlyForceRecursive = m.Spec.BindOptions.ReadOnlyForceRecursive
			}
			if m.Volume != nil {
				daemon.LogVolumeEvent(m.Volume.Name(), events.ActionMount, map[string]string{
					"driver":      m.Volume.DriverName(),
					"container":   c.ID,
					"destination": m.Destination,
					"read/write":  strconv.FormatBool(m.RW),
					"propagation": string(m.Propagation),
				})
			}
			mounts = append(mounts, mnt)
		}
	}

	mounts = sortMounts(mounts)
	netMounts := c.NetworkMounts()
	// if we are going to mount any of the network files from container
	// metadata, the ownership must be set properly for potential container
	// remapped root (user namespaces)
	rootIDs := daemon.idMapping.RootPair()
	for _, mnt := range netMounts {
		// we should only modify ownership of network files within our own container
		// metadata repository. If the user specifies a mount path external, it is
		// up to the user to make sure the file has proper ownership for userns
		if strings.Index(mnt.Source, daemon.repository) == 0 {
			if err := os.Chown(mnt.Source, rootIDs.UID, rootIDs.GID); err != nil {
				return nil, nil, err
			}
		}
	}
	return append(mounts, netMounts...), cleanups.Release(), nil
}

// sortMounts sorts an array of mounts in lexicographic order. This ensure that
// when mounting, the mounts don't shadow other mounts. For example, if mounting
// /etc and /etc/resolv.conf, /etc/resolv.conf must not be mounted first.
func sortMounts(m []container.Mount) []container.Mount {
	sort.Sort(mounts(m))
	return m
}

// setBindModeIfNull is platform specific processing to ensure the
// shared mode is set to 'z' if it is null. This is called in the case
// of processing a named volume and not a typical bind.
func setBindModeIfNull(bind *volumemounts.MountPoint) {
	if bind.Mode == "" {
		bind.Mode = "z"
	}
}
