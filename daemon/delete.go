package daemon

import (
	"fmt"
	"os"
	"path"

	"github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/volume/store"
)

// ContainerRmConfig is a holder for passing in runtime config.
type ContainerRmConfig struct {
	ForceRemove, RemoveVolume, RemoveLink bool
}

// ContainerRm removes the container id from the filesystem. An error
// is returned if the container is not found, or if the remove
// fails. If the remove succeeds, the container name is released, and
// network links are removed.
func (daemon *Daemon) ContainerRm(name string, config *ContainerRmConfig) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	if config.RemoveLink {
		name, err := GetFullContainerName(name)
		if err != nil {
			return err
		}
		parent, n := path.Split(name)
		if parent == "/" {
			return derr.ErrorCodeDefaultName
		}
		pe := daemon.containerGraph().Get(parent)
		if pe == nil {
			return derr.ErrorCodeNoParent.WithArgs(parent, name)
		}

		if err := daemon.containerGraph().Delete(name); err != nil {
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
		// return derr.ErrorCodeCantDestroy.WithArgs(name, utils.GetErrorMessage(err))
		return err
	}

	if err := container.removeMountPoints(config.RemoveVolume); err != nil {
		logrus.Error(err)
	}

	return nil
}

// Destroy unregisters a container from the daemon and cleanly removes its contents from the filesystem.
func (daemon *Daemon) rm(container *Container, forceRemove bool) (err error) {
	if container.IsRunning() {
		if !forceRemove {
			return derr.ErrorCodeRmRunning
		}
		if err := container.Kill(); err != nil {
			return derr.ErrorCodeRmFailed.WithArgs(err)
		}
	}

	// Container state RemovalInProgress should be used to avoid races.
	if err = container.setRemovalInProgress(); err != nil {
		if err == derr.ErrorCodeAlreadyRemoving {
			// do not fail when the removal is in progress started by other request.
			return nil
		}
		return derr.ErrorCodeRmState.WithArgs(err)
	}
	defer container.resetRemovalInProgress()

	// stop collection of stats for the container regardless
	// if stats are currently getting collected.
	daemon.statsCollector.stopCollection(container)

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
		return derr.ErrorCodeRmDriverFS.WithArgs(daemon.driver, container.ID, err)
	}

	initID := fmt.Sprintf("%s-init", container.ID)
	if err := daemon.driver.Remove(initID); err != nil {
		return derr.ErrorCodeRmInit.WithArgs(daemon.driver, initID, err)
	}

	if err = os.RemoveAll(container.root); err != nil {
		return derr.ErrorCodeRmFS.WithArgs(container.ID, err)
	}

	if err = daemon.execDriver.Clean(container.ID); err != nil {
		return derr.ErrorCodeRmExecDriver.WithArgs(container.ID, err)
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
		if err == store.ErrVolumeInUse {
			return derr.ErrorCodeRmVolumeInUse.WithArgs(err)
		}
		return derr.ErrorCodeRmVolume.WithArgs(name, err)
	}
	return nil
}
