package graph

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/utils"
)

const DEFAULTTAG = "latest"

type TagStore struct {
	path         string
	graph        *Graph
	Repositories map[string]Repository
	sync.Mutex
	// FIXME: move push/pull-related fields
	// to a helper type
	pullingPool map[string]chan struct{}
	pushingPool map[string]chan struct{}
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
		pullingPool:  make(map[string]chan struct{}),
		pushingPool:  make(map[string]chan struct{}),
	}
	// Load the json file if it exists, otherwise create it.
	if err := store.reload(); os.IsNotExist(err) {
		if err := store.save(); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return store, nil
}

func (store *TagStore) save() error {
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

func (store *TagStore) reload() error {
	jsonData, err := ioutil.ReadFile(store.path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(jsonData, store); err != nil {
		return err
	}
	return nil
}

func (store *TagStore) LookupImage(name string) (*image.Image, error) {
	// FIXME: standardize on returning nil when the image doesn't exist, and err for everything else
	// (so we can pass all errors here)
	repos, tag := parsers.ParseRepositoryTag(name)
	if tag == "" {
		tag = DEFAULTTAG
	}
	img, err := store.GetImage(repos, tag)
	store.Lock()
	defer store.Unlock()
	if err != nil {
		return nil, err
	} else if img == nil {
		if img, err = store.graph.Get(name); err != nil {
			return nil, err
		}
	}
	return img, nil
}

// Return a reverse-lookup table of all the names which refer to each image
// Eg. {"43b5f19b10584": {"base:latest", "base:v1"}}
func (store *TagStore) ByID() map[string][]string {
	store.Lock()
	defer store.Unlock()
	byID := make(map[string][]string)
	for repoName, repository := range store.Repositories {
		for tag, id := range repository {
			name := repoName + ":" + tag
			if _, exists := byID[id]; !exists {
				byID[id] = []string{name}
			} else {
				byID[id] = append(byID[id], name)
				sort.Strings(byID[id])
			}
		}
	}
	return byID
}

func (store *TagStore) ImageName(id string) string {
	if names, exists := store.ByID()[id]; exists && len(names) > 0 {
		return names[0]
	}
	return utils.TruncateID(id)
}

func (store *TagStore) DeleteAll(id string) error {
	names, exists := store.ByID()[id]
	if !exists || len(names) == 0 {
		return nil
	}
	for _, name := range names {
		if strings.Contains(name, ":") {
			nameParts := strings.Split(name, ":")
			if _, err := store.Delete(nameParts[0], nameParts[1]); err != nil {
				return err
			}
		} else {
			if _, err := store.Delete(name, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

func (store *TagStore) Delete(repoName, tag string) (bool, error) {
	store.Lock()
	defer store.Unlock()
	deleted := false
	if err := store.reload(); err != nil {
		return false, err
	}
	if r, exists := store.Repositories[repoName]; exists {
		if tag != "" {
			if _, exists2 := r[tag]; exists2 {
				delete(r, tag)
				if len(r) == 0 {
					delete(store.Repositories, repoName)
				}
				deleted = true
			} else {
				return false, fmt.Errorf("No such tag: %s:%s", repoName, tag)
			}
		} else {
			delete(store.Repositories, repoName)
			deleted = true
		}
	} else {
		return false, fmt.Errorf("No such repository: %s", repoName)
	}
	return deleted, store.save()
}

func (store *TagStore) Set(repoName, tag, imageName string, force bool) error {
	img, err := store.LookupImage(imageName)
	store.Lock()
	defer store.Unlock()
	if err != nil {
		return err
	}
	if tag == "" {
		tag = DEFAULTTAG
	}
	if err := validateRepoName(repoName); err != nil {
		return err
	}
	if err := validateTagName(tag); err != nil {
		return err
	}
	if err := store.reload(); err != nil {
		return err
	}
	var repo Repository
	if r, exists := store.Repositories[repoName]; exists {
		repo = r
	} else {
		repo = make(map[string]string)
		if old, exists := store.Repositories[repoName]; exists && !force {
			return fmt.Errorf("Conflict: Tag %s:%s is already set to %s", repoName, tag, old)
		}
		store.Repositories[repoName] = repo
	}
	repo[tag] = img.ID
	return store.save()
}

func (store *TagStore) Get(repoName string) (Repository, error) {
	store.Lock()
	defer store.Unlock()
	if err := store.reload(); err != nil {
		return nil, err
	}
	if r, exists := store.Repositories[repoName]; exists {
		return r, nil
	}
	return nil, nil
}

func (store *TagStore) GetImage(repoName, tagOrID string) (*image.Image, error) {
	repo, err := store.Get(repoName)
	store.Lock()
	defer store.Unlock()
	if err != nil {
		return nil, err
	} else if repo == nil {
		return nil, nil
	}
	if revision, exists := repo[tagOrID]; exists {
		return store.graph.Get(revision)
	}
	// If no matching tag is found, search through images for a matching image id
	for _, revision := range repo {
		if strings.HasPrefix(revision, tagOrID) {
			return store.graph.Get(revision)
		}
	}
	return nil, nil
}

func (store *TagStore) GetRepoRefs() map[string][]string {
	store.Lock()
	reporefs := make(map[string][]string)

	for name, repository := range store.Repositories {
		for tag, id := range repository {
			shortID := utils.TruncateID(id)
			reporefs[shortID] = append(reporefs[shortID], fmt.Sprintf("%s:%s", name, tag))
		}
	}
	store.Unlock()
	return reporefs
}

// Validate the name of a repository
func validateRepoName(name string) error {
	if name == "" {
		return fmt.Errorf("Repository name can't be empty")
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

func (s *TagStore) poolAdd(kind, key string) (chan struct{}, error) {
	s.Lock()
	defer s.Unlock()

	if c, exists := s.pullingPool[key]; exists {
		return c, fmt.Errorf("pull %s is already in progress", key)
	}
	if c, exists := s.pushingPool[key]; exists {
		return c, fmt.Errorf("push %s is already in progress", key)
	}

	c := make(chan struct{})
	switch kind {
	case "pull":
		s.pullingPool[key] = c
	case "push":
		s.pushingPool[key] = c
	default:
		return nil, fmt.Errorf("Unknown pool type")
	}
	return c, nil
}

func (s *TagStore) poolRemove(kind, key string) error {
	s.Lock()
	defer s.Unlock()
	switch kind {
	case "pull":
		if c, exists := s.pullingPool[key]; exists {
			close(c)
			delete(s.pullingPool, key)
		}
	case "push":
		if c, exists := s.pushingPool[key]; exists {
			close(c)
			delete(s.pushingPool, key)
		}
	default:
		return fmt.Errorf("Unknown pool type")
	}
	return nil
}
