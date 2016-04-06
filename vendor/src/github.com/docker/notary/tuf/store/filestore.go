package store

import (
	"fmt"
	"github.com/docker/notary"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

// NewFilesystemStore creates a new store in a directory tree
func NewFilesystemStore(baseDir, metaSubDir, metaExtension string) (*FilesystemStore, error) {
	metaDir := path.Join(baseDir, metaSubDir)

	// Make sure we can create the necessary dirs and they are writable
	err := os.MkdirAll(metaDir, 0700)
	if err != nil {
		return nil, err
	}

	return &FilesystemStore{
		baseDir:       baseDir,
		metaDir:       metaDir,
		metaExtension: metaExtension,
	}, nil
}

// FilesystemStore is a store in a locally accessible directory
type FilesystemStore struct {
	baseDir       string
	metaDir       string
	metaExtension string
}

func (f *FilesystemStore) getPath(name string) string {
	fileName := fmt.Sprintf("%s.%s", name, f.metaExtension)
	return filepath.Join(f.metaDir, fileName)
}

// GetMeta returns the meta for the given name (a role) up to size bytes
// If size is -1, this corresponds to "infinite," but we cut off at 100MB
func (f *FilesystemStore) GetMeta(name string, size int64) ([]byte, error) {
	meta, err := ioutil.ReadFile(f.getPath(name))
	if err != nil {
		if os.IsNotExist(err) {
			err = ErrMetaNotFound{Resource: name}
		}
		return nil, err
	}
	if size == -1 {
		size = notary.MaxDownloadSize
	}
	// Only return up to size bytes
	if int64(len(meta)) < size {
		return meta, nil
	}
	return meta[:size], nil
}

// SetMultiMeta sets the metadata for multiple roles in one operation
func (f *FilesystemStore) SetMultiMeta(metas map[string][]byte) error {
	for role, blob := range metas {
		err := f.SetMeta(role, blob)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetMeta sets the meta for a single role
func (f *FilesystemStore) SetMeta(name string, meta []byte) error {
	fp := f.getPath(name)

	// Ensures the parent directories of the file we are about to write exist
	err := os.MkdirAll(filepath.Dir(fp), 0700)
	if err != nil {
		return err
	}

	// if something already exists, just delete it and re-write it
	os.RemoveAll(fp)

	// Write the file to disk
	if err = ioutil.WriteFile(fp, meta, 0600); err != nil {
		return err
	}
	return nil
}

// RemoveAll clears the existing filestore by removing its base directory
func (f *FilesystemStore) RemoveAll() error {
	return os.RemoveAll(f.baseDir)
}

// RemoveMeta removes the metadata for a single role - if the metadata doesn't
// exist, no error is returned
func (f *FilesystemStore) RemoveMeta(name string) error {
	return os.RemoveAll(f.getPath(name)) // RemoveAll succeeds if path doesn't exist
}
