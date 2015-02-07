package daemon

import (
	"io"

	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerCopy(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("Usage: %s CONTAINER RESOURCE\n", job.Name)
	}

	var (
		name     = job.Args[0]
		resource = job.Args[1]
	)

	container, err := daemon.Get(name)
	if err != nil {
		return job.Error(err)
	}

	data, err := container.Copy(resource)
	if err != nil {
		return job.Error(err)
	}
	defer data.Close()

	if _, err := io.Copy(job.Stdout, data); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}
