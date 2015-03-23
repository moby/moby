package daemon

import (
	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerChanges(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: %s CONTAINER", job.Name)
	}
	name := job.Args[0]

	container, error := daemon.Get(name)
	if error != nil {
		return job.Error(error)
	}

	outs := engine.NewTable("", 0)
	changes, err := container.Changes()
	if err != nil {
		return job.Error(err)
	}

	for _, change := range changes {
		out := &engine.Env{}
		if err := out.Import(change); err != nil {
			return job.Error(err)
		}
		outs.Add(out)
	}

	if _, err := outs.WriteListTo(job.Stdout); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}
