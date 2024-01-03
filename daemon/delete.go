package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/log"
	"github.com/docker/docker/api/types/backend"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/containerfs"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
)

// ContainerRm removes the container id from the filesystem. An error
// is returned if the container is not found, or if the remove
// fails. If the remove succeeds, the container name is released, and
// network links are removed.
func (daemon *Daemon) ContainerRm(name string, config *backend.ContainerRmConfig) error {
	return daemon.containerRm(&daemon.config().Config, name, config)
}

func (daemon *Daemon) containerRm(cfg *config.Config, name string, opts *backend.ContainerRmConfig) error {
	start := time.Now()
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	// Container state RemovalInProgress should be used to avoid races.
	if inProgress := ctr.SetRemovalInProgress(); inProgress {
		err := fmt.Errorf("removal of container %s is already in progress", name)
		return errdefs.Conflict(err)
	}
	defer ctr.ResetRemovalInProgress()

	// check if container wasn't deregistered by previous rm since Get
	if c := daemon.containers.Get(ctr.ID); c == nil {
		return nil
	}

	if opts.RemoveLink {
		return daemon.rmLink(cfg, ctr, name)
	}

	err = daemon.cleanupContainer(ctr, *opts)
	containerActions.WithValues("delete").UpdateSince(start)

	return err
}

func (daemon *Daemon) rmLink(cfg *config.Config, container *container.Container, name string) error {
	if name[0] != '/' {
		name = "/" + name
	}
	parent, n := path.Split(name)
	if parent == "/" {
		return fmt.Errorf("Conflict, cannot remove the default link name of the container")
	}

	parent = strings.TrimSuffix(parent, "/")
	pe, err := daemon.containersReplica.Snapshot().GetID(parent)
	if err != nil {
		return fmt.Errorf("Cannot get parent %s for link name %s", parent, name)
	}

	daemon.releaseName(name)
	parentContainer, _ := daemon.GetContainer(pe)
	if parentContainer != nil {
		daemon.linkIndex.unlink(name, container, parentContainer)
		if err := daemon.updateNetwork(cfg, parentContainer); err != nil {
			log.G(context.TODO()).Debugf("Could not update network to remove link %s: %v", n, err)
		}
	}
	return nil
}

// cleanupContainer unregisters a container from the daemon, stops stats
// collection and cleanly removes contents and metadata from the filesystem.
func (daemon *Daemon) cleanupContainer(container *container.Container, config backend.ContainerRmConfig) error {
	if container.IsRunning() {
		if !config.ForceRemove {
			if state := container.StateString(); state == "paused" {
				return errdefs.Conflict(fmt.Errorf("cannot remove container %q: container is %s and must be unpaused first", container.Name, state))
			} else {
				return errdefs.Conflict(fmt.Errorf("cannot remove container %q: container is %s: stop the container before removing or force remove", container.Name, state))
			}
		}
		if err := daemon.Kill(container); err != nil && !isNotRunning(err) {
			return fmt.Errorf("cannot remove container %q: could not kill: %w", container.Name, err)
		}
	}

	// stop collection of stats for the container regardless
	// if stats are currently getting collected.
	daemon.statsCollector.StopCollection(container)

	// stopTimeout is the number of seconds to wait for the container to stop
	// gracefully before forcibly killing it.
	//
	// Why 3 seconds? The timeout specified here was originally added in commit
	// 1615bb08c7c3fc6c4b22db0a633edda516f97cf0, which added a custom timeout to
	// some commands, but lacking an option for a timeout on "docker rm", was
	// hardcoded to 10 seconds. Commit 28fd289b448164b77affd8103c0d96fd8110daf9
	// later on updated this to 3 seconds (but no background on that change).
	//
	// If you arrived here and know the answer, you earned yourself a picture
	// of a cute animal of your own choosing.
	stopTimeout := 3
	if err := daemon.containerStop(context.TODO(), container, containertypes.StopOptions{Timeout: &stopTimeout}); err != nil {
		return err
	}

	// Mark container dead. We don't want anybody to be restarting it.
	container.Lock()
	container.Dead = true

	// Save container state to disk. So that if error happens before
	// container meta file got removed from disk, then a restart of
	// docker should not make a dead container alive.
	if err := container.CheckpointTo(daemon.containersReplica); err != nil && !os.IsNotExist(err) {
		log.G(context.TODO()).Errorf("Error saving dying container to disk: %v", err)
	}
	container.Unlock()

	// When container creation fails and `RWLayer` has not been created yet, we
	// do not call `ReleaseRWLayer`
	if container.RWLayer != nil {
		if err := daemon.imageService.ReleaseLayer(container.RWLayer); err != nil {
			err = errors.Wrapf(err, "container %s", container.ID)
			container.SetRemovalError(err)
			return err
		}
		container.RWLayer = nil
	} else {
		if daemon.UsesSnapshotter() {
			ls := daemon.containerdClient.LeasesService()
			lease := leases.Lease{
				ID: container.ID,
			}
			if err := ls.Delete(context.Background(), lease, leases.SynchronousDelete); err != nil {
				if !cerrdefs.IsNotFound(err) {
					container.SetRemovalError(err)
					return err
				}
			}
		}
	}

	// Hold the container lock while deleting the container root directory
	// so that other goroutines don't attempt to concurrently open files
	// within it. Having any file open on Windows (without the
	// FILE_SHARE_DELETE flag) will block it from being deleted.
	container.Lock()
	err := containerfs.EnsureRemoveAll(container.Root)
	container.Unlock()
	if err != nil {
		err = errors.Wrapf(err, "unable to remove filesystem for %s", container.ID)
		container.SetRemovalError(err)
		return err
	}

	linkNames := daemon.linkIndex.delete(container)
	selinux.ReleaseLabel(container.ProcessLabel)
	daemon.containers.Delete(container.ID)
	daemon.containersReplica.Delete(container)
	if err := daemon.removeMountPoints(container, config.RemoveVolume); err != nil {
		log.G(context.TODO()).Error(err)
	}
	for _, name := range linkNames {
		daemon.releaseName(name)
	}
	container.SetRemoved()
	stateCtr.del(container.ID)

	daemon.LogContainerEvent(container, events.ActionDestroy)
	return nil
}
