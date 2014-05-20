package daemon

import (
	"encoding/json"

	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/runconfig"
)

func (daemon *Daemon) ContainerInspect(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("usage: %s NAME", job.Name)
	}
	name := job.Args[0]
	if container := daemon.Get(name); container != nil {
		b, err := json.Marshal(&struct {
			*Container
			HostConfig *runconfig.HostConfig
		}{container, container.HostConfig()})
		if err != nil {
			return job.Error(err)
		}
		job.Stdout.Write(b)
		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}
