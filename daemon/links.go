package daemon

import (
	"github.com/docker/docker/engine"
)

func (d *Daemon) RegisterLinkJobs(job *engine.Job) engine.Status {
	// register calls here
	return engine.StatusOK
}
