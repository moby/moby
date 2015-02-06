package daemon

import (
	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerRestart(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}
	var (
		name = job.Args[0]
		t    = 10
	)
	if job.EnvExists("t") {
		t = job.GetenvInt("t")
	}
	container, err := daemon.Get(name)
	if err != nil {
		return job.Error(err)
	}
	if err := container.Restart(int(t)); err != nil {
		return job.Errorf("Cannot restart container %s: %s\n", name, err)
	}
	container.LogEvent("restart")
	return engine.StatusOK
}
