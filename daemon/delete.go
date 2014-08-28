package daemon

import (
	"fmt"
	"os"
	"path"

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
	container := daemon.Get(name)

	if container == nil {
		job.Errorf("No such container: %s", name)
	}

	if removeLink {
		name, err := GetFullContainerName(name)
		if err != nil {
			job.Error(err)
		}
		parent, n := path.Split(name)
		if parent == "/" {
			return job.Errorf("Conflict, cannot remove the default name of the container")
		}
		pe := daemon.ContainerGraph().Get(parent)
		if pe == nil {
			return job.Errorf("Cannot get parent %s for name %s", parent, name)
		}
		parentContainer := daemon.Get(pe.ID())

		if parentContainer != nil {
			parentContainer.DisableLink(n)
		}

		if err := daemon.ContainerGraph().Delete(name); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	}

	if container.State.IsRunning() {
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
		volumes, err := container.GetVolumes()
		if err != nil {
			return job.Errorf("Could not get volumes for container %s: %s", name, err)
		}
		daemon.RemoveVolumes(volumes)
	}
	return engine.StatusOK
}

func (daemon *Daemon) RemoveVolumes(volumes map[string]*Volume) {
	for _, v := range volumes {
		if v.isBindMount {
			continue
		}
		if err := daemon.volumes.CanRemove(v); err != nil {
			log.Infof("%v", err)
			continue
		}
		if err := daemon.volumes.Delete(v.Id()); err != nil {
			// Do not return here just because 1 volume couldn't be deleted
			// Just log the event
			log.Errorf("Error calling volumes.Delete(%q): %v", v.Id(), err)
		}

		daemon.volumes.Remove(v)
	}
	return
}

// Destroy unregisters a container from the daemon and cleanly removes its contents from the filesystem.
// FIXME: rename to Rm for consistency with the CLI command
func (daemon *Daemon) Destroy(container *Container) error {
	if container == nil {
		return fmt.Errorf("The given container is <nil>")
	}

	element := daemon.containers.Get(container.ID)
	if element == nil {
		return fmt.Errorf("Container %v not found - maybe it was already destroyed?", container.ID)
	}

	if err := container.Stop(3); err != nil {
		return err
	}

	// Deregister the container before removing its directory, to avoid race conditions
	daemon.idIndex.Delete(container.ID)
	daemon.containers.Delete(container.ID)
	container.derefVolumes()

	if _, err := daemon.containerGraph.Purge(container.ID); err != nil {
		log.Debugf("Unable to remove container from link graph: %s", err)
	}

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

	selinuxFreeLxcContexts(container.ProcessLabel)

	return nil
}
