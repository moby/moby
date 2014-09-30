package daemon

import (
	"errors"

	"github.com/docker/docker/engine"
)

func (d *Daemon) RegisterLinkJobs(eng *engine.Engine) error {
	eng.Register("link_add", d.linkAddJob)
	eng.Register("link_remove", d.linkRemoveJob)
	return nil
}

func (d *Daemon) linkAddJob(job *engine.Job) engine.Status {
	if len(job.Args) < 3 {
		return job.Error(errors.New("`docker links add`: not enough arguments"))
	}

	parent := d.Get(job.Args[0])
	child := d.Get(job.Args[1])

	if parent == nil || child == nil {
		return job.Error(errors.New("`docker links add`: invalid container name specified"))
	}

	if job.Args[2] == "" {
		return job.Error(errors.New("Alias cannot be empty"))
	}

	if err := d.RegisterLink(parent, child, job.Args[2]); err != nil {
		return job.Error(err)
	}

	child.UpdateParentsHosts()

	return engine.StatusOK
}

func (d *Daemon) linkRemoveJob(job *engine.Job) engine.Status {
	if len(job.Args) < 3 {
		return job.Error(errors.New("`docker links remove`: not enough arguments"))
	}

	parent := d.Get(job.Args[0])
	child := d.Get(job.Args[1])

	if parent == nil || child == nil {
		return job.Error(errors.New("`docker links remove`: invalid container name specified"))
	}

	if err := d.UnregisterLink(parent, child, job.Args[2]); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}
