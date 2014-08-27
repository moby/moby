package links

import (
	"github.com/docker/docker/engine"
)

func (lm *Links) RegisterJobs(eng *engine.Engine) error {
	return eng.RegisterMap(map[string]engine.Handler{
		"create_link": lm.createLink,
		"purge_link":  lm.purgeLink,
	})
}

func (nm *Names) RegisterJobs(eng *engine.Engine) error {
	return eng.RegisterMap(map[string]engine.Handler{
		"create_name": nm.createName,
		"get_name":    nm.getName,
		"delete_name": nm.deleteName,
	})
}

func (lm *Links) createLink(job *engine.Job) engine.Status {
	var (
		parentName = job.Getenv("ParentName")
		childId    = job.Getenv("ChildID")
		alias      = job.Getenv("Alias")
	)

	if err := lm.Create(parentName, childId, alias); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

func (lm *Links) purgeLink(job *engine.Job) engine.Status {
	name := job.Getenv("Name")

	if err := lm.Purge(name); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

func (nm *Names) createName(job *engine.Job) engine.Status {
	var (
		name = job.Getenv("Name")
		id   = job.Getenv("ID")
	)

	if err := nm.Create(name, id); err != nil {
		job.Setenv("ErrorMsg", err.Error())
		return job.Error(err)
	}

	return engine.StatusOK
}

func (nm *Names) getName(job *engine.Job) engine.Status {
	name := job.Getenv("Name")

	res, err := nm.Get(name)

	if err != nil {
		return job.Error(err)
	}

	job.Setenv("Result", res)

	return engine.StatusOK
}

func (nm *Names) deleteName(job *engine.Job) engine.Status {
	name := job.Getenv("Name")

	if err := nm.Delete(name); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}
