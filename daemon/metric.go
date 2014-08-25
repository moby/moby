package daemon

import (
	"encoding/json"

	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerMetric(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}

	var name = job.Args[0]

	if container := daemon.Get(name); container != nil {
		if !container.State.IsRunning() {
			return job.Errorf("Container %s is not running", name)
		}

		metric, err := daemon.GetMetric(container)
		if err != nil {
			return job.Error(err)
		}

		b, err := json.Marshal(metric)
		if err != nil {
			return job.Error(err)
		}
		job.Stdout.Write(b)

		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}
