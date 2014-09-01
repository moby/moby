package links

import (
	"errors"
	"fmt"

	"github.com/docker/docker/pkg/graphdb"
)

var ErrDuplicateName = errors.New("Conflict: name already exists.")

type Names struct {
	dbPath         string
	containerGraph *graphdb.Database
}

func NewNames(dbpath string) (*Names, error) {
	containerGraph, err := graphdb.NewSqliteConn(dbpath)
	return &Names{dbpath, containerGraph}, err
}

func (nm *Names) Close() error {
	return nm.containerGraph.Close()
}

func (nm *Names) Create(name, id string) error {
	_, err := nm.containerGraph.Set(name, id)

	if err != nil && graphdb.IsNonUniqueNameError(err) {
		return ErrDuplicateName
	}

	return err
}

func (nm *Names) Get(name string) (string, error) {
	entity := nm.containerGraph.Get(name)

	if entity == nil {
		return "", fmt.Errorf("Could not find entity for %s", name)
	}

	return entity.ID(), nil
}

func (nm *Names) Delete(name string) error {
	return nm.containerGraph.Delete(name)
}

func (nm *Names) Each(query string, queryFunc func(string, string) error) error {
	entities := nm.containerGraph.List(query, -1)

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
