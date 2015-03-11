package daemon

import "github.com/docker/docker/engine"

func (daemon *Daemon) ContainerRename(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("usage: %s OLD_NAME NEW_NAME", job.Name)
	}
	oldName := job.Args[0]
	newName := job.Args[1]

	container, err := daemon.Get(oldName)
	if err != nil {
		return job.Error(err)
	}

	oldName = container.Name

	container.Lock()
	defer container.Unlock()
	if newName, err = daemon.reserveName(container.ID, newName); err != nil {
		return job.Errorf("Error when allocating new name: %s", err)
	}

	container.Name = newName

	undo := func() {
		container.Name = oldName
		daemon.reserveName(container.ID, oldName)
		daemon.containerGraph.Delete(newName)
	}

	if err := daemon.containerGraph.Delete(oldName); err != nil {
		undo()
		return job.Errorf("Failed to delete container %q: %v", oldName, err)
	}

	if err := container.toDisk(); err != nil {
		undo()
		return job.Error(err)
	}

	return engine.StatusOK
}
