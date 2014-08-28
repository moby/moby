package links

import (
	"path"

	"github.com/docker/docker/pkg/graphdb"
)

type Links struct {
	dbPath         string
	containerGraph *graphdb.Database
}

func NewLinks(dbpath string) (*Links, error) {
	containerGraph, err := graphdb.NewSqliteConn(dbpath)
	return &Links{dbpath, containerGraph}, err
}

func (lm *Links) Close() error {
	return lm.containerGraph.Close()
}

func (lm *Links) Create(parentPath, childId, alias string) error {
	fullPath := path.Join(parentPath, alias)
	if !lm.containerGraph.Exists(fullPath) {
		_, err := lm.containerGraph.Set(fullPath, childId)
		return err
	}
	return nil
}

func (lm *Links) Purge(name string) error {
	_, err := lm.containerGraph.Purge(name)
	return err
}

func (lm *Links) MapChildren(name string, applyFunc func(string, string) error) error {
	return lm.containerGraph.Walk(name, func(p string, e *graphdb.Entity) error {
		return applyFunc(p, e.ID())
	}, 0)
}

func (lm *Links) Children(name string) (map[string]string, error) {
	children := map[string]string{}

	err := lm.MapChildren(name, func(path, id string) error {
		children[path] = id
		return nil
	})

	return children, err
}

func (lm *Links) Parents(name string) ([]string, error) {
	return lm.containerGraph.Parents(name)
}
