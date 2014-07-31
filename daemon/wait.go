package daemon

import (
	"time"

	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerWait(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s", job.Name)
	}
	name := job.Args[0]
	if container := daemon.Get(name); container != nil {
		status, _ := container.State.WaitStop(-1 * time.Second)
		job.Printf("%d\n", status)
		return engine.StatusOK
	}
	return job.Errorf("%s: no such container: %s", job.Name, name)
}
