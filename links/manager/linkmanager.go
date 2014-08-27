package manager

import (
	"path"

	"github.com/docker/docker/pkg/graphdb"
)

type LinkManager struct {
	dbPath         string
	containerGraph *graphdb.Database
}

func NewLinkManager(dbpath string) (*LinkManager, error) {
	containerGraph, err := graphdb.NewSqliteConn(dbpath)
	return &LinkManager{dbpath, containerGraph}, err
}

func (lm *LinkManager) Close() error {
	return lm.containerGraph.Close()
}

func (lm *LinkManager) Create(parentPath, childId, alias string) error {
	fullPath := path.Join(parentPath, alias)
	if !lm.containerGraph.Exists(fullPath) {
		_, err := lm.containerGraph.Set(fullPath, childId)
		return err
	}
	return nil
}

func (lm *LinkManager) Purge(name string) error {
	_, err := lm.containerGraph.Purge(name)
	return err
}

func (lm *LinkManager) MapChildren(name string, applyFunc func(string, string) error) error {
	return lm.containerGraph.Walk(name, func(p string, e *graphdb.Entity) error {
		return applyFunc(p, e.ID())
	}, 0)
}

func (lm *LinkManager) Children(name string) (map[string]string, error) {
	children := map[string]string{}

	err := lm.MapChildren(name, func(path, id string) error {
		children[path] = id
		return nil
	})

	return children, err
}

func (lm *LinkManager) Parents(name string) ([]string, error) {
	return lm.containerGraph.Parents(name)
}
