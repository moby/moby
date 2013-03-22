package docker

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
)

type TagStore struct {
	path         string
	graph        *Graph
	Repositories map[string]Repository
}

type Repository map[string]string

func NewTagStore(path string, graph *Graph) (*TagStore, error) {
	abspath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	store := &TagStore{
		path:         abspath,
		graph:        graph,
		Repositories: make(map[string]Repository),
	}
	if err := store.Save(); err != nil {
		return nil, err
	}
	return store, nil
}

func (store *TagStore) Save() error {
	// Store the json ball
	jsonData, err := json.Marshal(store)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(store.path, jsonData, 0600); err != nil {
		return err
	}
	return nil
}

func (store *TagStore) Reload() error {
	jsonData, err := ioutil.ReadFile(store.path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(jsonData, store); err != nil {
		return err
	}
	return nil
}

func (store *TagStore) Set(repoName, tag, revision string) error {
	if err := store.Reload(); err != nil {
		return err
	}
	var repo Repository
	if r, exists := store.Repositories[repoName]; exists {
		repo = r
	} else {
		repo = make(map[string]string)
		store.Repositories[repoName] = repo
	}
	repo[tag] = revision
	return store.Save()
}

func (store *TagStore) Get(repoName string) (Repository, error) {
	if err := store.Reload(); err != nil {
		return nil, err
	}
	if r, exists := store.Repositories[repoName]; exists {
		return r, nil
	}
	return nil, nil
}

func (store *TagStore) GetImage(repoName, tag string) (*Image, error) {
	repo, err := store.Get(repoName)
	if err != nil {
		return nil, err
	} else if repo == nil {
		return nil, nil
	}
	if revision, exists := repo[tag]; exists {
		return store.graph.Get(revision)
	}
	return nil, nil
}
