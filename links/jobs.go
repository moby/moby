package links

import (
	"fmt"

	"github.com/docker/docker/engine"
)

func (lm *Links) Install(eng *engine.Engine) error {
	return eng.RegisterMap(map[string]engine.Handler{
		"create_link":    lm.createLink,
		"purge_link":     lm.purgeLink,
		"parents_link":   lm.listLinks,
		"create_name":    lm.createName,
		"get_name":       lm.getName,
		"delete_name":    lm.deleteName,
		"list_entities":  lm.listEntities,
		"list_parents":   lm.listParents,
		"close_links_db": lm.closeDb,
	})
}

func (lm *Links) closeDb(job *engine.Job) engine.Status {
	if err := lm.Close(); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

func (lm *Links) listParents(job *engine.Job) engine.Status {
	parents, err := lm.Parents(job.Args[0])
	if err != nil {
		return job.Error(err)
	}

	job.SetenvJson("Parents", parents)
	return engine.StatusOK
}

func (lm *Links) listEntities(job *engine.Job) engine.Status {
	var (
		query    = job.Args[0]
		entities = lm.containerGraph.List(query, -1)
		result   = map[string]string{}
	)

	if entities == nil {
		return job.Error(fmt.Errorf("No entities for query %s", query))
	}

	for _, p := range entities.Paths() {
		result[p] = entities[p].ID()
	}

	job.SetenvJson("Result", result)

	return engine.StatusOK
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
