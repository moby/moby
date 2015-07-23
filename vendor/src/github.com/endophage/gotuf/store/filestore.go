package store

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

func NewFilesystemStore(baseDir, metaSubDir, metaExtension, targetsSubDir string) (*filesystemStore, error) {
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

	return &filesystemStore{
		baseDir:       baseDir,
		metaDir:       metaDir,
		metaExtension: metaExtension,
		targetsDir:    targetsDir,
	}, nil
}

type filesystemStore struct {
	baseDir       string
	metaDir       string
	metaExtension string
	targetsDir    string
}

func (f *filesystemStore) GetMeta(name string, size int64) ([]byte, error) {
	fileName := fmt.Sprintf("%s.%s", name, f.metaExtension)
	path := filepath.Join(f.metaDir, fileName)
	meta, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return meta, nil
}

func (f *filesystemStore) SetMultiMeta(metas map[string][]byte) error {
	for role, blob := range metas {
		err := f.SetMeta(role, blob)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *filesystemStore) SetMeta(name string, meta []byte) error {
	fileName := fmt.Sprintf("%s.%s", name, f.metaExtension)
	path := filepath.Join(f.metaDir, fileName)
	if err := ioutil.WriteFile(path, meta, 0600); err != nil {
		return err
	}
	return nil
}
