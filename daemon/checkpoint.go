package daemon

import (
	"github.com/docker/docker/engine"
)

// Checkpoint a running container.
func (daemon *Daemon) ContainerCheckpoint(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}

	name := job.Args[0]
	container, err := daemon.Get(name)
	if err != nil {
		return job.Error(err)
	}
	if !container.IsRunning() {
		return job.Errorf("Container %s not running", name)
	}

	if err := container.Checkpoint(); err != nil {
		return job.Errorf("Cannot checkpoint container %s: %s", name, err)
	}

	container.LogEvent("checkpoint")
	return engine.StatusOK
}

// Restore a checkpointed container.
func (daemon *Daemon) ContainerRestore(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}

	name := job.Args[0]
	container, err := daemon.Get(name)
	if err != nil {
		return job.Error(err)
	}
	if container.IsRunning() {
		return job.Errorf("Container %s already running", name)
	}
	if !container.State.IsCheckpointed() {
		return job.Errorf("Container %s is not checkpointed", name)
	}

	if err := container.Restore(); err != nil {
		container.LogEvent("die")
		return job.Errorf("Cannot restore container %s: %s", name, err)
	}

	container.LogEvent("restore")
	return engine.StatusOK
}
