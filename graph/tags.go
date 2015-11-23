package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/daemon/events"
	"github.com/docker/docker/graph/tags"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/broadcaster"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
	"github.com/docker/libtrust"
)

// ErrNameIsNotExist returned when there is no image with requested name.
var ErrNameIsNotExist = errors.New("image with specified name does not exist")

// TagStore manages repositories. It encompasses the Graph used for versioned
// storage, as well as various services involved in pushing and pulling
// repositories.
type TagStore struct {
	path  string
	graph *Graph
	// Repositories is a map of repositories, indexed by name.
	Repositories map[string]Repository
	trustKey     libtrust.PrivateKey
	sync.Mutex
	// FIXME: move push/pull-related fields
	// to a helper type
	pullingPool     map[string]*broadcaster.Buffered
	pushingPool     map[string]*broadcaster.Buffered
	registryService *registry.Service
	eventsService   *events.Events
	ConfirmDefPush  bool
}

// Repository maps tags to image IDs.
type Repository map[string]string

// Update updates repository mapping with content of repository 'u'.
func (r Repository) Update(u Repository) {
	for k, v := range u {
		r[k] = v
	}
}

// Contains returns true if the contents of Repository u are wholly contained
// in Repository r.
func (r Repository) Contains(u Repository) bool {
	for k, v := range u {
		// if u's key is not present in r OR u's key is present, but not the same value
		if rv, ok := r[k]; !ok || (ok && rv != v) {
			return false
		}
	}
	return true
}

// TagStoreConfig provides parameters for a new TagStore.
type TagStoreConfig struct {
	// Graph is the versioned image store
	Graph *Graph
	// Key is the private key to use for signing manifests.
	Key libtrust.PrivateKey
	// Registry is the registry service to use for TLS configuration and
	// endpoint lookup.
	Registry *registry.Service
	// Events is the events service to use for logging.
	Events *events.Events
	// Whether to ask user to confirm a push to public Docker registry.
	ConfirmDefPush bool
}

// NewTagStore creates a new TagStore at specified path, using the parameters
// and services provided in cfg.
func NewTagStore(path string, cfg *TagStoreConfig) (*TagStore, error) {
	abspath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	store := &TagStore{
		path:            abspath,
		graph:           cfg.Graph,
		trustKey:        cfg.Key,
		Repositories:    make(map[string]Repository),
		pullingPool:     make(map[string]*broadcaster.Buffered),
		pushingPool:     make(map[string]*broadcaster.Buffered),
		registryService: cfg.Registry,
		eventsService:   cfg.Events,
		ConfirmDefPush:  cfg.ConfirmDefPush,
	}
	// Load the json file if it exists, otherwise create it.
	if err := store.reload(); os.IsNotExist(err) {
		if err := store.save(); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if store.ConfirmDefPush != cfg.ConfirmDefPush {
		store.ConfirmDefPush = cfg.ConfirmDefPush
		if err := store.save(); err != nil {
			logrus.Warnf("Failed to write TagStore configuration to %s: %v", abspath, err)
		}
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
	f, err := os.Open(store.path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&store); err != nil {
		return err
	}
	return nil
}

// LookupImage returns pointer to an Image struct corresponding to the given
// name. The name can include an optional tag; otherwise the default tag will
// be used.
func (store *TagStore) LookupImage(name string) (*image.Image, error) {
	// FIXME: standardize on returning nil when the image doesn't exist, and err for everything else
	// (so we can pass all errors here)
	repoName, ref := parsers.ParseRepositoryTag(name)
	if ref == "" {
		ref = tags.DefaultTag
	}
	var (
		err error
		img *image.Image
	)

	img, err = store.GetImage(repoName, ref)
	if err != nil {
		return nil, err
	}

	if img != nil {
		return img, nil
	}

	// name must be an image ID.
	store.Lock()
	defer store.Unlock()
	if img, err = store.graph.Get(name); err != nil {
		return nil, err
	}

	return img, nil
}

// NormalizeLocalName returns a local name for given image if it doesn't
// exist. Id of existing local image won't be changed.
func (store *TagStore) NormalizeLocalName(name string) string {
	if _, exists := store.Repositories[name]; exists {
		return name
	}
	if _, err := store.graph.idIndex.Get(name); err == nil {
		return name
	}
	return registry.NormalizeLocalName(name)
}

// GetID returns ID for image name.
func (store *TagStore) GetID(name string) (string, error) {
	repoName, ref := parsers.ParseRepositoryTag(name)
	if ref == "" {
		ref = tags.DefaultTag
	}
	store.Lock()
	defer store.Unlock()
	matching := store.getRepositoryList(repoName)
	for _, repo := range matching {
		for _, repoRefs := range repo {
			if id, exists := repoRefs[ref]; exists {
				return id, nil
			}
		}
	}
	return "", ErrNameIsNotExist
}

// ByID returns a reverse-lookup table of all the names which refer to each
// image - e.g. {"43b5f19b10584": {"base:latest", "base:v1"}}
func (store *TagStore) ByID() map[string][]string {
	store.Lock()
	defer store.Unlock()
	byID := make(map[string][]string)
	for repoName, repository := range store.Repositories {
		for tag, id := range repository {
			name := utils.ImageReference(repoName, tag)
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

// HasReferences returns whether or not the given image is referenced in one or
// more repositories.
func (store *TagStore) HasReferences(img *image.Image) bool {
	return len(store.ByID()[img.ID]) > 0
}

// ImageName returns name of an image, given the image's ID.
func (store *TagStore) ImageName(id string) string {
	if names, exists := store.ByID()[id]; exists && len(names) > 0 {
		return names[0]
	}
	return stringid.TruncateID(id)
}

// DeleteAll removes images identified by a specific ID from the store.
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

// Delete deletes a repository or a specific tag. If ref is empty, the entire
// repository named repoName will be deleted; otherwise only the tag named by
// ref will be deleted.
func (store *TagStore) Delete(repoName, ref string) (bool, error) {
	store.Lock()
	defer store.Unlock()
	err := store.reload()
	if err != nil {
		return false, err
	}

	matching := store.getRepositoryList(repoName)
	for _, namedRepo := range matching {
		deleted := false
		for name, repoRefs := range namedRepo {
			if ref == "" {
				// Delete the whole repository.
				delete(store.Repositories, name)
				deleted = true
				break
			}

			if _, exists := repoRefs[ref]; exists {
				delete(repoRefs, ref)
				if len(repoRefs) == 0 {
					delete(store.Repositories, name)
				}
				deleted = true
				break
			}
			err = fmt.Errorf("No such reference: %s:%s", repoName, ref)
		}
		if deleted {
			return true, store.save()
		}
	}

	if err != nil {
		return false, err
	}
	return false, fmt.Errorf("No such repository: %s", repoName)
}

// Tag creates a tag in the repository reponame, pointing to the image named
// imageName. If force is true, an existing tag with the same name may be
// overwritten. Default repository will be prepend to unqualified repoName
// unless keepUnqualified is true.
func (store *TagStore) Tag(repoName, tag, imageName string, force, keepUnqualified bool) error {
	return store.setLoad(repoName, tag, imageName, force, keepUnqualified, nil)
}

// setLoad stores the image to the store.
// If the imageName is already in the repo then a '-f' flag should be used to replace existing image.
func (store *TagStore) setLoad(repoName, tag, imageName string, force, keepUnqualified bool, out io.Writer) error {
	img, err := store.LookupImage(imageName)
	store.Lock()
	defer store.Unlock()
	if err != nil {
		return err
	}
	if tag == "" {
		tag = tags.DefaultTag
	}
	if err := validateRepoName(repoName); err != nil {
		return err
	}
	if err := tags.ValidateTagName(tag); err != nil {
		return err
	}
	if err := store.reload(); err != nil {
		return err
	}
	var repo Repository
	normalized := registry.NormalizeLocalName(repoName)
	if keepUnqualified && !registry.RepositoryNameHasIndex(repoName) {
		_, normalized = registry.SplitReposName(normalized, false)
	}
	if r, exists := store.Repositories[normalized]; exists {
		repo = r
		if old, exists := store.Repositories[normalized][tag]; exists {

			if !force {
				return fmt.Errorf("Conflict: Tag %s:%s is already set to image %s, if you want to replace it, please use -f option", repoName, tag, old[:12])
			}

			if old != img.ID && out != nil {

				fmt.Fprintf(out, "The image %s:%s already exists, renaming the old one with ID %s to empty string\n", normalized, tag, old[:12])

			}
		}
	} else {
		repo = make(map[string]string)
		store.Repositories[normalized] = repo
	}
	repo[tag] = img.ID
	return store.save()
}

// SetDigest creates a digest reference to an image ID.
func (store *TagStore) SetDigest(repoName, digest, imageName string, keepUnqualified bool) error {
	img, err := store.LookupImage(imageName)
	if err != nil {
		return err
	}

	if err := validateRepoName(repoName); err != nil {
		return err
	}

	if err := validateDigest(digest); err != nil {
		return err
	}

	store.Lock()
	defer store.Unlock()
	if err := store.reload(); err != nil {
		return err
	}

	normalized := registry.NormalizeLocalName(repoName)
	if keepUnqualified && !registry.RepositoryNameHasIndex(repoName) {
		_, normalized = registry.SplitReposName(normalized, false)
	}
	repoRefs, exists := store.Repositories[normalized]
	if !exists {
		repoRefs = Repository{}
		store.Repositories[normalized] = repoRefs
	} else if oldID, exists := repoRefs[digest]; exists && oldID != img.ID {
		return fmt.Errorf("Conflict: Digest %s is already set to image %s", digest, oldID)
	}

	repoRefs[digest] = img.ID
	return store.save()
}

// getRepository returnes a list of local repositories matching given
// repository name. If repository is fully qualified, there will be one match
// at the most. Otherwise results will be sorted in following way:
//   1. precise match
//   2. precise match after normalization
//   3. match after prefixing with default registry name and normalization
//   4. match against remote name of repository prefixed with non-default registry
// *Default registry* here means any registry in registry.RegistryList.
// Returned is a list of maps with just one entry {"repositoryName": Repository}
func (store *TagStore) getRepositoryList(repoName string) (result []map[string]Repository) {
	repoMap := map[string]struct{}{}
	addResult := func(name string, repo Repository) bool {
		if _, exists := repoMap[name]; exists {
			return false
		}
		result = append(result, map[string]Repository{name: repo})
		repoMap[name] = struct{}{}
		return true
	}
	if r, exists := store.Repositories[repoName]; exists {
		addResult(repoName, r)
	}
	if r, exists := store.Repositories[registry.NormalizeLocalName(repoName)]; exists {
		addResult(registry.NormalizeLocalName(repoName), r)
	}
	if !registry.RepositoryNameHasIndex(repoName) {
		defaultRegistries := make(map[string]struct{}, len(registry.RegistryList))
		for i := 0; i < len(registry.RegistryList); i++ {
			defaultRegistries[registry.RegistryList[i]] = struct{}{}
			if i < 1 {
				continue
			}
			fqn := registry.NormalizeLocalName(registry.RegistryList[i] + "/" + repoName)
			if r, exists := store.Repositories[fqn]; exists {
				addResult(fqn, r)
			}
		}
		for name, r := range store.Repositories {
			indexName, remoteName := registry.SplitReposName(name, false)
			if indexName != "" && remoteName == repoName {
				if _, exists := defaultRegistries[indexName]; exists {
					addResult(name, r)
				}
			}
		}
	}
	return
}

// Get returns the Repository tag/image map for a given repository.
func (store *TagStore) Get(repoName string) (Repository, error) {
	store.Lock()
	defer store.Unlock()
	if err := store.reload(); err != nil {
		return nil, err
	}
	matching := store.getRepositoryList(repoName)
	if len(matching) > 0 {
		for _, repoRefs := range matching[0] {
			return repoRefs, nil
		}
	}
	return nil, nil
}

// GetImage returns a pointer to an Image structure describing the image
// referred to by refOrID inside repository repoName.
func (store *TagStore) GetImage(repoName, refOrID string) (*image.Image, error) {
	store.Lock()
	defer store.Unlock()

	matching := store.getRepositoryList(repoName)
	for _, namedRepo := range matching {
		for _, repoRefs := range namedRepo {
			if revision, exists := repoRefs[refOrID]; exists {
				return store.graph.Get(revision)
			}
		}
	}
	// If no matching reference is found, search through images for a matching
	// image id if it looks like a short ID or would look like a short ID.
	if stringid.IsShortID(stringid.TruncateID(refOrID)) {
		for _, namedRepo := range matching {
			for _, repoRefs := range namedRepo {
				for _, revision := range repoRefs {
					if strings.HasPrefix(revision, refOrID) {
						return store.graph.Get(revision)
					}
				}
			}
		}
	}

	return nil, nil
}

// GetRepoRefs returns a map with image IDs as keys, and slices listing
// repo/tag references as the values. It covers all repositories.
func (store *TagStore) GetRepoRefs() map[string][]string {
	store.Lock()
	reporefs := make(map[string][]string)

	for name, repository := range store.Repositories {
		for tag, id := range repository {
			shortID := stringid.TruncateID(id)
			reporefs[shortID] = append(reporefs[shortID], utils.ImageReference(name, tag))
		}
	}
	store.Unlock()
	return reporefs
}

// validateRepoName validates the name of a repository.
func validateRepoName(name string) error {
	if name == "" {
		return fmt.Errorf("Repository name can't be empty")
	}
	if strings.TrimPrefix(strings.TrimPrefix(name, registry.IndexName+"/"), "library/") == "scratch" {
		return fmt.Errorf("'scratch' is a reserved name")
	}
	return nil
}

func validateDigest(dgst string) error {
	if dgst == "" {
		return errors.New("digest can't be empty")
	}
	if _, err := digest.ParseDigest(dgst); err != nil {
		return err
	}
	return nil
}

// poolAdd checks if a push or pull is already running, and returns
// (broadcaster, true) if a running operation is found. Otherwise, it creates a
// new one and returns (broadcaster, false).
func (store *TagStore) poolAdd(kind, key string) (*broadcaster.Buffered, bool) {
	store.Lock()
	defer store.Unlock()

	if p, exists := store.pullingPool[key]; exists {
		return p, true
	}
	if p, exists := store.pushingPool[key]; exists {
		return p, true
	}

	broadcaster := broadcaster.NewBuffered()

	switch kind {
	case "pull":
		store.pullingPool[key] = broadcaster
	case "push":
		store.pushingPool[key] = broadcaster
	default:
		panic("Unknown pool type")
	}

	return broadcaster, false
}

func (store *TagStore) poolRemoveWithError(kind, key string, broadcasterResult error) error {
	store.Lock()
	defer store.Unlock()
	switch kind {
	case "pull":
		if broadcaster, exists := store.pullingPool[key]; exists {
			broadcaster.CloseWithError(broadcasterResult)
			delete(store.pullingPool, key)
		}
	case "push":
		if broadcaster, exists := store.pushingPool[key]; exists {
			broadcaster.CloseWithError(broadcasterResult)
			delete(store.pushingPool, key)
		}
	default:
		return fmt.Errorf("Unknown pool type")
	}
	return nil
}

func (store *TagStore) poolRemove(kind, key string) error {
	return store.poolRemoveWithError(kind, key, nil)
}
