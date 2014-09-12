package links

import (
	"fmt"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/nat"
)

func (lm *Links) Install(eng *engine.Engine) error {
	lm.engine = eng
	return eng.RegisterMap(map[string]engine.Handler{
		"get_link_env": lm.getLinkEnv,
		"enable_link":  lm.enableLink,
		"disable_link": lm.disableLink,
		"create_link":  lm.createLink,
		"get_name":     lm.getName,
	})
}

func (lm *Links) getLinkEnv(job *engine.Job) engine.Status {
	var (
		parentIP = job.Getenv("ParentIP")
		childIP  = job.Getenv("ChildIP")
		name     = job.Getenv("Name")
	)

	if _, ok := lm.activeLinks[name]; ok {
		if link, ok := lm.activeLinks[name][combineIP(parentIP, childIP)]; ok {
			env := link.ToEnv()
			job.SetenvList("Result", env)
			return engine.StatusOK
		}
	}

	return job.Error(fmt.Errorf("Link %s, addrs %s not found", name, combineIP(parentIP, childIP)))
}

func (lm *Links) enableLink(job *engine.Job) engine.Status {
	var (
		parentIP = job.Getenv("ParentIP")
		childIP  = job.Getenv("ChildIP")
		name     = job.Getenv("Name")
		env      = job.GetenvList("ChildEnvironment")
		ports    = job.GetenvList("Ports")
	)

	portMap := map[nat.Port]struct{}{}

	for _, port := range ports {
		portMap[nat.Port(port)] = struct{}{}
	}

	link, err := NewLink(parentIP, childIP, name, env, portMap, lm.engine)

	if err != nil {
		return job.Error(err)
	}

	if lm.activeLinks[name] == nil {
		lm.activeLinks[name] = map[string]*Link{}
	}

	lm.activeLinks[name][parentIP+" "+childIP] = link

	if err := link.Enable(); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

func (lm *Links) disableLink(job *engine.Job) engine.Status {
	var (
		parentIP = job.Getenv("ParentIP")
		childIP  = job.Getenv("ChildIP")
		name     = job.Getenv("Name")
	)

	if _, ok := lm.activeLinks[name]; ok {
		if link, ok := lm.activeLinks[name][combineIP(parentIP, childIP)]; ok {
			link.Disable()
			delete(lm.activeLinks[name], combineIP(parentIP, childIP))
		}
	}

	return engine.StatusOK
}

func (lm *Links) createLink(job *engine.Job) engine.Status {
	var (
		parentName = job.Getenv("ParentName")
		childId    = job.Getenv("ChildID")
	)

	if err := lm.CreateLink(parentName, childId); err != nil {
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
