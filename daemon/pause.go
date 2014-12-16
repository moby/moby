package daemon

import (
	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerPause(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER", job.Name)
	}
	name := job.Args[0]
	container, err := daemon.Get(name)
	if err != nil {
		return job.Error(err)
	}
	if err := container.Pause(); err != nil {
		return job.Errorf("Cannot pause container %s: %s", name, err)
	}
	container.LogEvent("pause")
	return engine.StatusOK
}

func (daemon *Daemon) ContainerUnpause(job *engine.Job) engine.Status {
	if n := len(job.Args); n < 1 || n > 2 {
		return job.Errorf("Usage: %s CONTAINER", job.Name)
	}
	name := job.Args[0]
	container, err := daemon.Get(name)
	if err != nil {
		return job.Error(err)
	}
	if err := container.Unpause(); err != nil {
		return job.Errorf("Cannot unpause container %s: %s", name, err)
	}
	container.LogEvent("unpause")
	return engine.StatusOK
}
