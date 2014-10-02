package daemon

import (
	"fmt"
	"os"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/log"
)

func (daemon *Daemon) ContainerRm(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Not enough arguments. Usage: %s CONTAINER\n", job.Name)
	}
	name := job.Args[0]
	removeVolume := job.GetenvBool("removeVolume")
	removeLink := job.GetenvBool("removeLink")
	forceRemove := job.GetenvBool("forceRemove")

	if removeLink {
		parent, child, alias, err := daemon.containers.DeconstructPath(name)
		if err != nil {
			return job.Error(err)
		}

		parent.DisableLink(alias)
		daemon.containers.Unlink(child)
		child.UpdateParentsHosts()

		return engine.StatusOK
	}

	container := daemon.Get(name)

	if container != nil {
		if container.IsRunning() {
			if forceRemove {
				if err := container.Kill(); err != nil {
					return job.Errorf("Could not kill running container, cannot remove - %v", err)
				}
			} else {
				return job.Errorf("You cannot remove a running container. Stop the container before attempting removal or use -f")
			}
		}
		if err := daemon.Destroy(container); err != nil {
			return job.Errorf("Cannot destroy container %s: %s", name, err)
		}
		container.LogEvent("destroy")
		if removeVolume {
			daemon.DeleteVolumes(container.VolumePaths())
		}
	}
	return engine.StatusOK
}

func (daemon *Daemon) DeleteVolumes(volumeIDs map[string]struct{}) {
	for id := range volumeIDs {
		if err := daemon.volumes.Delete(id); err != nil {
			log.Infof("%s", err)
			continue
		}
	}
}

// Destroy unregisters a container from the daemon and cleanly removes its contents from the filesystem.
// FIXME: rename to Rm for consistency with the CLI command
func (daemon *Daemon) Destroy(container *Container) error {
	if container == nil {
		return fmt.Errorf("The given container is <nil>")
	}

	if _, err := daemon.containers.GetByID(container.ID); err != nil {
		return err
	}

	if err := container.Stop(3); err != nil {
		return err
	}

	daemon.containers.Delete(container)

	if err := daemon.driver.Remove(container.ID); err != nil {
		return fmt.Errorf("Driver %s failed to remove root filesystem %s: %s", daemon.driver, container.ID, err)
	}

	initID := fmt.Sprintf("%s-init", container.ID)
	if err := daemon.driver.Remove(initID); err != nil {
		return fmt.Errorf("Driver %s failed to remove init filesystem %s: %s", daemon.driver, initID, err)
	}

	if err := os.RemoveAll(container.root); err != nil {
		return fmt.Errorf("Unable to remove filesystem for %v: %v", container.ID, err)
	}

	if err := daemon.execDriver.Clean(container.ID); err != nil {
		return fmt.Errorf("Unable to remove execdriver data for %s: %s", container.ID, err)
	}

	selinuxFreeLxcContexts(container.ProcessLabel)

	return nil
}
