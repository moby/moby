package daemon

import (
	"path"

	"github.com/docker/docker/pkg/graphdb"
)

type LinkManager struct {
	containerGraph *graphdb.Database
}

func NewLinkManager(containerGraph *graphdb.Database) *LinkManager {
	return &LinkManager{containerGraph}
}

func (lm *LinkManager) Create(parent, child *Container, alias string) error {
	fullName := path.Join(parent.Name, alias)
	if !lm.containerGraph.Exists(fullName) {
		_, err := lm.containerGraph.Set(fullName, child.ID)
		return err
	}
	return nil
}

func (lm *LinkManager) Get()    {}
func (lm *LinkManager) Update() {}
func (lm *LinkManager) Delete() {}

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
