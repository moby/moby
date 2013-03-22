package docker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const DEFAULT_TAG = "latest"

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
	// Load the json file if it exists, otherwise create it.
	if err := store.Reload(); os.IsNotExist(err) {
		if err := store.Save(); err != nil {
			return nil, err
		}
	} else if err != nil {
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

func (store *TagStore) LookupImage(name string) (*Image, error) {
	img, err := store.graph.Get(name)
	if err != nil {
		// FIXME: standardize on returning nil when the image doesn't exist, and err for everything else
		// (so we can pass all errors here)
		repoAndTag := strings.SplitN(name, ":", 2)
		if len(repoAndTag) == 1 {
			repoAndTag = append(repoAndTag, DEFAULT_TAG)
		}
		if i, err := store.GetImage(repoAndTag[0], repoAndTag[1]); err != nil {
			return nil, err
		} else if i == nil {
			return nil, fmt.Errorf("No such image: %s", name)
		} else {
			img = i
		}
	}
	return img, nil
}

func (store *TagStore) Set(repoName, tag, revision string) error {
	if tag == "" {
		tag = DEFAULT_TAG
	}
	if err := validateRepoName(repoName); err != nil {
		return err
	}
	if err := validateTagName(tag); err != nil {
		return err
	}
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

// Validate the name of a repository
func validateRepoName(name string) error {
	if name == "" {
		return fmt.Errorf("Repository name can't be empty")
	}
	if strings.Contains(name, ":") {
		return fmt.Errorf("Illegal repository name: %s", name)
	}
	return nil
}

// Validate the name of a tag
func validateTagName(name string) error {
	if name == "" {
		return fmt.Errorf("Tag name can't be empty")
	}
	if strings.Contains(name, "/") || strings.Contains(name, ":") {
		return fmt.Errorf("Illegal tag name: %s", name)
	}
	return nil
}
