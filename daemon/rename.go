package daemon

import (
	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerRename(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("usage: %s OLD_NAME NEW_NAME", job.Name)
	}
	old_name := job.Args[0]
	new_name := job.Args[1]

	container := daemon.Get(old_name)
	if container == nil {
		return job.Errorf("No such container: %s", old_name)
	}

	container.Lock()
	defer container.Unlock()
	if err := daemon.containerGraph.Delete(container.Name); err != nil {
		return job.Errorf("Failed to delete container %q: %v", old_name, err)
	}
	if _, err := daemon.reserveName(container.ID, new_name); err != nil {
		return job.Errorf("Error when allocating new name: %s", err)
	}
	container.Name = new_name

	return engine.StatusOK
}
