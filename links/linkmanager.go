package links

import (
	"os"

	"github.com/docker/docker/daemon"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/graphdb"
)

type Links struct {
	daemon      *daemon.Daemon
	activeLinks map[string]map[string]*Link
	engine      *engine.Engine
}

func combineIP(parent, child string) string {
	return parent + " " + child
}

func linksMigrate(d *daemon.Daemon, containerGraph *graphdb.Database) error {
	found := map[string]*daemon.Container{}
	containerGraph.Walk("/", func(p string, e *graphdb.Entity) error {
		container := d.Get(e.ID())
		if container != nil {
			container.Name = p[1:]
			found[container.Name] = container
			container.ToDisk()
		}
		return nil
	}, 0)

	for name, container := range found {
		children, err := containerGraph.Children("/"+name, 1)
		if err != nil {
			return err
		}

		for _, child := range children {
			if childContainer, ok := found[child.Edge.Name]; ok {
				if container.LinkMap == nil {
					container.LinkMap = map[string]string{}
				}
				container.LinkMap[child.Edge.Name] = childContainer.ID
			}
		}

		container.ToDisk()
	}

	return nil
}

func NewLinks(dbpath string, d *daemon.Daemon) (*Links, error) {
	if _, err := os.Stat(dbpath); err == nil {
		containerGraph, err := graphdb.NewSqliteConn(dbpath)
		if err != nil {
			return nil, err
		}

		if err := linksMigrate(d, containerGraph); err != nil {
			containerGraph.Close()
			return nil, err
		} else {
			containerGraph.Close()
			os.Remove(dbpath) // remove the db now that we've successfully migrated it
		}
	}

	return &Links{d, map[string]map[string]*Link{}, nil}, nil
}

func (lm *Links) CreateLink(name, id string) error {
	parent, err := lm.daemon.GetByName(name)
	if err != nil {
		return err
	}

	child := lm.daemon.Get(id)

	if child != nil {
		if parent.LinkMap == nil {
			parent.LinkMap = map[string]string{}
		}
		parent.LinkMap[child.Name] = id
		parent.ToDisk()
		return nil
	}

	return err
}

func (lm *Links) GetID(name string) (string, error) {
	container, err := lm.daemon.GetByName(name)
	if err != nil {
		return "", err
	}

	return container.ID, nil
}
