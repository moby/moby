package contextstore

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

const metadataDir = "meta"
const metaFile = "meta.json"

type metadataStore struct {
	root string
}

func (s *metadataStore) contextDir(name string) string {
	return filepath.Join(s.root, name)
}

func (s *metadataStore) createOrUpdate(name string, meta ContextMetadata) error {
	contextDir := s.contextDir(name)
	err := os.MkdirAll(contextDir, 0755)
	if err != nil {
		return err
	}
	bytes, err := json.Marshal(&meta)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(contextDir, metaFile), bytes, 0644)
}

func (s *metadataStore) get(name string) (ContextMetadata, error) {
	contextDir := s.contextDir(name)
	bytes, err := ioutil.ReadFile(filepath.Join(contextDir, metaFile))
	if err != nil {
		return ContextMetadata{}, err
	}
	var r ContextMetadata
	err = json.Unmarshal(bytes, &r)
	return r, err
}

func (s *metadataStore) remove(name string) error {
	contextDir := s.contextDir(name)
	return os.RemoveAll(contextDir)
}

func (s *metadataStore) list() (map[string]ContextMetadata, error) {
	ctxNames, err := listRecursivelyMetadataDirs(s.root)
	if err != nil {
		return nil, err
	}
	res := make(map[string]ContextMetadata)
	for _, name := range ctxNames {
		res[name], err = s.get(name)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func isContextDir(path string) bool {
	s, err := os.Stat(filepath.Join(path, metaFile))
	if err != nil {
		return false
	}
	return !s.IsDir()
}

func listRecursivelyMetadataDirs(root string) ([]string, error) {
	fis, err := ioutil.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, fi := range fis {
		if fi.IsDir() {
			if isContextDir(filepath.Join(root, fi.Name())) {
				result = append(result, fi.Name())
			}
			subs, err := listRecursivelyMetadataDirs(filepath.Join(root, fi.Name()))
			if err != nil {
				return nil, err
			}
			for _, s := range subs {
				result = append(result, fmt.Sprintf("%s/%s", fi.Name(), s))
			}
		}
	}
	return result, nil
}

// EndpointMetadata contains metadata about an endpoint
type EndpointMetadata map[string]interface{}

// ContextMetadata contains metadata about a context and its endpoints
type ContextMetadata struct {
	Metadata  map[string]interface{}      `json:"metadata,omitempty"`
	Endpoints map[string]EndpointMetadata `json:"endpoints,omitempty"`
}
