package graph

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
)

type RepoStore struct {
	path         string
	graph        *Graph
	Repositories map[string]*Repository
}

type Repository struct {
	Tags map[string]string
}

func NewRepoStore(path string, graph *Graph) (*RepoStore, error) {
	abspath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	store := &RepoStore{
		path:         abspath,
		graph:        graph,
		Repositories: make(map[string]*Repository),
	}
	if err := store.Reload(); err != nil {
		return nil, err
	}
	return store, nil
}

func (store *RepoStore) Save() error {
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

func (store *RepoStore) Reload() error {
	jsonData, err := ioutil.ReadFile(store.path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(jsonData, store); err != nil {
		return err
	}
	return nil
}

func (store *RepoStore) SetTag(repoName, tag, revision string) error {
	if err := store.Reload(); err != nil {
		return err
	}
	var repo *Repository
	if r, exists := store.Repositories[repoName]; exists {
		repo = r
	} else {
		repo = NewRepository()
		store.Repositories[repoName] = repo
	}
	repo.Tags[tag] = revision
	return store.Save()
}

func (store *RepoStore) Get(repoName string) (*Repository, error) {
	if err := store.Reload(); err != nil {
		return nil, err
	}
	if r, exists := store.Repositories[repoName]; exists {
		return r, nil
	}
	return nil, nil
}

func (store *RepoStore) GetImage(repoName, tag string) (*Image, error) {
	repo, err := store.Get(repoName)
	if err != nil {
		return nil, err
	} else if repo == nil {
		return nil, nil
	}
	if revision, exists := repo.Tags[tag]; exists {
		return store.graph.Get(revision)
	}
	return nil, nil
}

func NewRepository() *Repository {
	return &Repository{
		Tags: make(map[string]string),
	}
}
