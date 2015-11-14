package store

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

// NewFilesystemStore creates a new store in a directory tree
func NewFilesystemStore(baseDir, metaSubDir, metaExtension, targetsSubDir string) (*FilesystemStore, error) {
	metaDir := path.Join(baseDir, metaSubDir)
	targetsDir := path.Join(baseDir, targetsSubDir)

	// Make sure we can create the necessary dirs and they are writable
	err := os.MkdirAll(metaDir, 0700)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(targetsDir, 0700)
	if err != nil {
		return nil, err
	}

	return &FilesystemStore{
		baseDir:       baseDir,
		metaDir:       metaDir,
		metaExtension: metaExtension,
		targetsDir:    targetsDir,
	}, nil
}

// FilesystemStore is a store in a locally accessible directory
type FilesystemStore struct {
	baseDir       string
	metaDir       string
	metaExtension string
	targetsDir    string
}

// GetMeta returns the meta for the given name (a role)
func (f *FilesystemStore) GetMeta(name string, size int64) ([]byte, error) {
	fileName := fmt.Sprintf("%s.%s", name, f.metaExtension)
	path := filepath.Join(f.metaDir, fileName)
	meta, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return meta, nil
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
	fileName := fmt.Sprintf("%s.%s", name, f.metaExtension)
	path := filepath.Join(f.metaDir, fileName)
	if err := ioutil.WriteFile(path, meta, 0600); err != nil {
		return err
	}
	return nil
}
