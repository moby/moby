package daemon

import "github.com/docker/docker/engine"

func (daemon *Daemon) ContainerSweep(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}
	var (
		name = job.Args[0]
	)

	if container := daemon.Get(name); container != nil {
		if !container.State.IsRunning() {
			return job.Errorf("Container already stopped")
		}
		if err := container.Sweep(job.Eng); err != nil {
			return job.Errorf("Cannot cleanup container %s: %s\n", name, err)
		}
		container.LogEvent("sweep")
	} else {
		return job.Errorf("No such container: %s\n", name)
	}
	return engine.StatusOK
}
