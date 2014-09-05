package links

import (
	"errors"
	"fmt"
	"path"

	"github.com/docker/docker/pkg/graphdb"
)

type Links struct {
	dbPath         string
	containerGraph *graphdb.Database
}

var ErrDuplicateName = errors.New("Conflict: name already exists.")

func NewLinks(dbpath string) (*Links, error) {
	containerGraph, err := graphdb.NewSqliteConn(dbpath)
	return &Links{dbpath, containerGraph}, err
}

func (lm *Links) Close() error {
	return lm.containerGraph.Close()
}

func (lm *Links) CreateName(name, id string) error {
	_, err := lm.containerGraph.Set(name, id)

	if err != nil && graphdb.IsNonUniqueNameError(err) {
		return ErrDuplicateName
	}

	return err
}

func (lm *Links) CreateLink(parentPath, childId, alias string) error {
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

func (lm *Links) Each(query string, queryFunc func(string, string) error) error {
	entities := lm.containerGraph.List(query, -1)

	if entities == nil {
		return nil
	}

	for _, p := range entities.Paths() {
		if err := queryFunc(p, entities[p].ID()); err != nil {
			return err
		}
	}

	return nil
}
func (lm *Links) GetID(name string) (string, error) {
	entity := lm.containerGraph.Get(name)

	if entity == nil {
		return "", fmt.Errorf("Could not find entity for %s", name)
	}

	return entity.ID(), nil
}

func (lm *Links) Delete(name string) error {
	return lm.containerGraph.Delete(name)
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

func (lm *Links) Links() (map[string][]string, error) {
	result := map[string][]string{}
	entities := lm.containerGraph.List("/", -1)

	if entities == nil {
		return result, nil
	}

	for p, e := range entities {
		if _, ok := result[p]; ok {
			result[p] = append(result[p], e.ID())
		} else {
			result[p] = []string{e.ID()}
		}
	}

	return result, nil
}
