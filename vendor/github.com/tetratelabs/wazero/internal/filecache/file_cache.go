package filecache

import (
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
)

// New returns a new Cache implemented by fileCache.
func New(dir string) Cache {
	return newFileCache(dir)
}

func newFileCache(dir string) *fileCache {
	return &fileCache{dirPath: dir}
}

// fileCache persists compiled functions into dirPath.
//
// Note: this can be expanded to do binary signing/verification, set TTL on each entry, etc.
type fileCache struct {
	dirPath string
}

func (fc *fileCache) path(key Key) string {
	return path.Join(fc.dirPath, hex.EncodeToString(key[:]))
}

func (fc *fileCache) Get(key Key) (content io.ReadCloser, ok bool, err error) {
	f, err := os.Open(fc.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	} else {
		return f, true, nil
	}
}

func (fc *fileCache) Add(key Key, content io.Reader) (err error) {
	path := fc.path(key)
	dirPath, fileName := filepath.Split(path)

	file, err := os.CreateTemp(dirPath, fileName+".*.tmp")
	if err != nil {
		return
	}
	defer func() {
		file.Close()
		if err != nil {
			_ = os.Remove(file.Name())
		}
	}()
	if _, err = io.Copy(file, content); err != nil {
		return
	}
	if err = file.Sync(); err != nil {
		return
	}
	if err = file.Close(); err != nil {
		return
	}
	err = os.Rename(file.Name(), path)
	return
}

func (fc *fileCache) Delete(key Key) (err error) {
	err = os.Remove(fc.path(key))
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return
}
