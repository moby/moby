package daemon

import (
	"strconv"

	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerResize(job *engine.Job) engine.Status {
	if len(job.Args) != 3 {
		return job.Errorf("Not enough arguments. Usage: %s CONTAINER HEIGHT WIDTH\n", job.Name)
	}
	name := job.Args[0]
	height, err := strconv.Atoi(job.Args[1])
	if err != nil {
		return job.Error(err)
	}
	width, err := strconv.Atoi(job.Args[2])
	if err != nil {
		return job.Error(err)
	}
	if container := daemon.Get(name); container != nil {
		if err := container.Resize(height, width); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}
