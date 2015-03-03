package blobstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type localStore struct {
	sync.Mutex
	root string
}

// NewLocalStore creates a new local blob store in the given root directory.
func NewLocalStore(root string) (Store, error) {
	return newLocalStore(root)
}

func (ls *localStore) blobDirname(digest string) string {
	return filepath.Join(ls.root, "blobs", digest)
}

func (ls *localStore) blobFilename(digest string) string {
	return filepath.Join(ls.blobDirname(digest), "blob")
}

func (ls *localStore) blobInfoFilename(digest string) string {
	return filepath.Join(ls.blobDirname(digest), "info.json")
}

// newLocalStore is the unexported version of NewLocalStore which returns
// the unexported type.
func newLocalStore(root string) (*localStore, error) {
	ls := &localStore{root: root}

	blobsDirname := ls.blobDirname("")
	if err := os.MkdirAll(blobsDirname, os.FileMode(0755)); err != nil {
		return nil, fmt.Errorf("unable to create local blob store directory %q: %s", blobsDirname, err)
	}

	return ls, nil
}
