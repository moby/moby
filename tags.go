package docker

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
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
			return nil, fmt.Errorf("Image does not exist: %s", name)
		} else {
			img = i
		}
	}
	return img, nil
}

// Return a reverse-lookup table of all the names which refer to each image
// Eg. {"43b5f19b10584": {"base:latest", "base:v1"}}
func (store *TagStore) ById() map[string][]string {
	byId := make(map[string][]string)
	for repoName, repository := range store.Repositories {
		for tag, id := range repository {
			name := repoName + ":" + tag
			if _, exists := byId[id]; !exists {
				byId[id] = []string{name}
			} else {
				byId[id] = append(byId[id], name)
				sort.Strings(byId[id])
			}
		}
	}
	return byId
}

func (store *TagStore) ImageName(id string) string {
	if names, exists := store.ById()[id]; exists && len(names) > 0 {
		return names[0]
	}
	return utils.TruncateId(id)
}

func (store *TagStore) Set(repoName, tag, imageName string, force bool) error {
	img, err := store.LookupImage(imageName)
	if err != nil {
		return err
	}
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
		if old, exists := store.Repositories[repoName]; exists && !force {
			return fmt.Errorf("Tag %s:%s is already set to %s", repoName, tag, old)
		}
		store.Repositories[repoName] = repo
	}
	repo[tag] = img.Id
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
