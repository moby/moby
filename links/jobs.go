package links

import "github.com/docker/docker/engine"

func (lm *Links) RegisterJobs(eng *engine.Engine) error {
	return eng.RegisterMap(map[string]engine.Handler{
		"create_link":  lm.createLink,
		"purge_link":   lm.purgeLink,
		"parents_link": lm.listLinks,
		"create_name":  lm.createName,
		"get_name":     lm.getName,
		"delete_name":  lm.deleteName,
	})
}

func (lm *Links) listLinks(job *engine.Job) engine.Status {
	links, err := lm.Links()
	if err != nil {
		return job.Error(err)
	}

	job.SetenvJson("Parents", links)

	return engine.StatusOK
}

func (lm *Links) createLink(job *engine.Job) engine.Status {
	var (
		parentName = job.Getenv("ParentName")
		childId    = job.Getenv("ChildID")
		alias      = job.Getenv("Alias")
	)

	if err := lm.CreateLink(parentName, childId, alias); err != nil {
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

func (lm *Links) createName(job *engine.Job) engine.Status {
	var (
		name = job.Getenv("Name")
		id   = job.Getenv("ID")
	)

	if err := lm.CreateName(name, id); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

func (lm *Links) getName(job *engine.Job) engine.Status {
	name := job.Getenv("Name")

	res, err := lm.GetID(name)

	if err != nil {
		return job.Error(err)
	}

	job.Setenv("Result", res)

	return engine.StatusOK
}

func (lm *Links) deleteName(job *engine.Job) engine.Status {
	name := job.Getenv("Name")

	if err := lm.Delete(name); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}
