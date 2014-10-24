package daemon

import (
	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerModify(job *engine.Job) engine.Status {
	if len(job.Args) != 3 {
		return job.Errorf("Usage: %s CONTAINER TYPE VALUES", job.Name)
	}
	name := job.Args[0]
	modifytype := job.Args[1]
	modifyvalues := job.Args[2]
	container := daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}
	if err := container.Modify(modifytype, modifyvalues); err != nil {
		return job.Errorf("Cannot apply modification to container %s: %s", name, err)
	}
	job.Eng.Job("log", "modify", container.ID, daemon.Repositories().ImageName(container.Image)).Run()
	return engine.StatusOK
}
