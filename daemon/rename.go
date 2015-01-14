package daemon

import (
	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerRename(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("usage: %s OLD_NAME NEW_NAME", job.Name)
	}
	oldName := job.Args[0]
	newName := job.Args[1]

	container := daemon.Get(oldName)
	if container == nil {
		return job.Errorf("No such container: %s", oldName)
	}

	oldName = container.Name

	container.Lock()
	defer container.Unlock()
	if _, err := daemon.reserveName(container.ID, newName); err != nil {
		return job.Errorf("Error when allocating new name: %s", err)
	}

	container.Name = newName

	if err := daemon.containerGraph.Delete(oldName); err != nil {
		return job.Errorf("Failed to delete container %q: %v", oldName, err)
	}

	return engine.StatusOK
}
