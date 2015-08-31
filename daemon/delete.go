package daemon

import (
	"fmt"
	"os"
	"path"

	"github.com/Sirupsen/logrus"
)

// ContainerRmConfig is a holder for passing in runtime config.
type ContainerRmConfig struct {
	// FullName is the container name prefixed with a slash, used to clean linked containers in the graph
	FullName string
	// ForceRemove tells whether it should remove the container regardless side effects or not
	ForceRemove bool
	// ForceVolume tells whether it should remove volumes attached to the container or not
	RemoveVolume bool
	// RemoveLink tells whether it should clean linked containers in the graph or not
	RemoveLink bool
}

// ContainerRm removes the container id from the filesystem. An error
// is returned if the container is not found, or if the remove
// fails. If the remove succeeds, the container name is released, and
// network links are removed.
func (daemon *Daemon) ContainerRm(container *Container, config *ContainerRmConfig) error {
	if config.RemoveLink {
		parent, n := path.Split(config.FullName)
		if parent == "/" {
			return fmt.Errorf("Conflict, cannot remove the default name of the container")
		}
		pe := daemon.containerGraph().Get(parent)
		if pe == nil {
			return fmt.Errorf("Cannot get parent %s for name %s", parent, config.FullName)
		}

		if err := daemon.containerGraph().Delete(config.FullName); err != nil {
			return err
		}

		parentContainer, _ := daemon.Get(pe.ID())
		if parentContainer != nil {
			if err := parentContainer.updateNetwork(); err != nil {
				logrus.Debugf("Could not update network to remove link %s: %v", n, err)
			}
		}

		return nil
	}

	if err := daemon.rm(container, config.ForceRemove); err != nil {
		return fmt.Errorf("Cannot destroy container %s: %v", container.ID, err)
	}

	container.removeMountPoints(config.RemoveVolume)
	return nil
}

// Destroy unregisters a container from the daemon and cleanly removes its contents from the filesystem.
func (daemon *Daemon) rm(container *Container, forceRemove bool) (err error) {
	if container.IsRunning() {
		if !forceRemove {
			return fmt.Errorf("Conflict, You cannot remove a running container. Stop the container before attempting removal or use -f")
		}
		if err := container.Kill(); err != nil {
			return fmt.Errorf("Could not kill running container, cannot remove - %v", err)
		}
	}

	// stop collection of stats for the container regardless
	// if stats are currently getting collected.
	daemon.statsCollector.stopCollection(container)

	element := daemon.containers.Get(container.ID)
	if element == nil {
		return fmt.Errorf("Container %v not found - maybe it was already destroyed?", container.ID)
	}

	// Container state RemovalInProgress should be used to avoid races.
	if err = container.setRemovalInProgress(); err != nil {
		return fmt.Errorf("Failed to set container state to RemovalInProgress: %s", err)
	}

	defer container.resetRemovalInProgress()

	if err = container.Stop(3); err != nil {
		return err
	}

	// Mark container dead. We don't want anybody to be restarting it.
	container.setDead()

	// Save container state to disk. So that if error happens before
	// container meta file got removed from disk, then a restart of
	// docker should not make a dead container alive.
	if err := container.toDiskLocking(); err != nil {
		logrus.Errorf("Error saving dying container to disk: %v", err)
	}

	// If force removal is required, delete container from various
	// indexes even if removal failed.
	defer func() {
		if err != nil && forceRemove {
			daemon.idIndex.Delete(container.ID)
			daemon.containers.Delete(container.ID)
			os.RemoveAll(container.root)
			container.logEvent("destroy")
		}
	}()

	if _, err := daemon.containerGraphDB.Purge(container.ID); err != nil {
		logrus.Debugf("Unable to remove container from link graph: %s", err)
	}

	if err = daemon.driver.Remove(container.ID); err != nil {
		return fmt.Errorf("Driver %s failed to remove root filesystem %s: %s", daemon.driver, container.ID, err)
	}

	initID := fmt.Sprintf("%s-init", container.ID)
	if err := daemon.driver.Remove(initID); err != nil {
		return fmt.Errorf("Driver %s failed to remove init filesystem %s: %s", daemon.driver, initID, err)
	}

	if err = os.RemoveAll(container.root); err != nil {
		return fmt.Errorf("Unable to remove filesystem for %v: %v", container.ID, err)
	}

	if err = daemon.execDriver.Clean(container.ID); err != nil {
		return fmt.Errorf("Unable to remove execdriver data for %s: %s", container.ID, err)
	}

	selinuxFreeLxcContexts(container.ProcessLabel)
	daemon.idIndex.Delete(container.ID)
	daemon.containers.Delete(container.ID)

	container.logEvent("destroy")
	return nil
}

// VolumeRm removes the volume with the given name.
// If the volume is referenced by a container it is not removed
// This is called directly from the remote API
func (daemon *Daemon) VolumeRm(name string) error {
	v, err := daemon.volumes.Get(name)
	if err != nil {
		return err
	}
	if err := daemon.volumes.Remove(v); err != nil {
		if err == ErrVolumeInUse {
			return fmt.Errorf("Conflict: %v", err)
		}
		return err
	}
	return nil
}
