package store

import (
	"bytes"
	"fmt"
	"io"

	"github.com/endophage/gotuf/data"
	"github.com/endophage/gotuf/errors"
	"github.com/endophage/gotuf/utils"
)

func NewMemoryStore(meta map[string][]byte, files map[string][]byte) *memoryStore {
	if meta == nil {
		meta = make(map[string][]byte)
	}
	if files == nil {
		files = make(map[string][]byte)
	}
	return &memoryStore{
		meta:  meta,
		files: files,
		keys:  make(map[string][]data.PrivateKey),
	}
}

type memoryStore struct {
	meta  map[string][]byte
	files map[string][]byte
	keys  map[string][]data.PrivateKey
}

func (m *memoryStore) GetMeta(name string, size int64) ([]byte, error) {
	d, ok := m.meta[name]
	if ok {
		if int64(len(d)) < size {
			return d, nil
		}
		return d[:size], nil
	} else {
		return nil, ErrMetaNotFound{}
	}
}

func (m *memoryStore) SetMeta(name string, meta []byte) error {
	m.meta[name] = meta
	return nil
}

func (m *memoryStore) SetMultiMeta(metas map[string][]byte) error {
	for role, blob := range metas {
		m.SetMeta(role, blob)
	}
	return nil
}

func (m *memoryStore) GetTarget(path string) (io.ReadCloser, error) {
	return &utils.NoopCloser{Reader: bytes.NewReader(m.files[path])}, nil
}

func (m *memoryStore) WalkStagedTargets(paths []string, targetsFn targetsWalkFunc) error {
	if len(paths) == 0 {
		for path, dat := range m.files {
			meta, err := data.NewFileMeta(bytes.NewReader(dat), "sha256")
			if err != nil {
				return err
			}
			if err = targetsFn(path, meta); err != nil {
				return err
			}
		}
		return nil
	}

	for _, path := range paths {
		dat, ok := m.files[path]
		if !ok {
			return errors.ErrFileNotFound{path}
		}
		meta, err := data.NewFileMeta(bytes.NewReader(dat), "sha256")
		if err != nil {
			return err
		}
		if err = targetsFn(path, meta); err != nil {
			return err
		}
	}
	return nil
}

func (m *memoryStore) Commit(map[string][]byte, bool, map[string]data.Hashes) error {
	return nil
}

func (m *memoryStore) GetKey(role string) ([]byte, error) {
	return nil, fmt.Errorf("GetKey is not implemented for the memoryStore")
}
