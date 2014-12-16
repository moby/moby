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
	container, err := daemon.Get(name)
	if err != nil {
		return job.Error(err)
	}
	if err := container.Resize(height, width); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func (daemon *Daemon) ContainerExecResize(job *engine.Job) engine.Status {
	if len(job.Args) != 3 {
		return job.Errorf("Not enough arguments. Usage: %s EXEC HEIGHT WIDTH\n", job.Name)
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
	execConfig, err := daemon.getExecConfig(name)
	if err != nil {
		return job.Error(err)
	}
	if err := execConfig.Resize(height, width); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}
