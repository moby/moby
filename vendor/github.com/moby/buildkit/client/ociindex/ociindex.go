package ociindex

import (
	"encoding/json"
	"io"
	"os"
	"path"

	"github.com/gofrs/flock"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	// lockFileSuffix is the suffix of the lock file
	lockFileSuffix = ".lock"
)

type StoreIndex struct {
	indexPath  string
	lockPath   string
	layoutPath string
}

func NewStoreIndex(storePath string) StoreIndex {
	indexPath := path.Join(storePath, ocispecs.ImageIndexFile)
	layoutPath := path.Join(storePath, ocispecs.ImageLayoutFile)
	return StoreIndex{
		indexPath:  indexPath,
		lockPath:   indexPath + lockFileSuffix,
		layoutPath: layoutPath,
	}
}

func (s StoreIndex) Read() (*ocispecs.Index, error) {
	lock := flock.New(s.lockPath)
	locked, err := lock.TryRLock()
	if err != nil {
		return nil, errors.Wrapf(err, "could not lock %s", s.lockPath)
	}
	if !locked {
		return nil, errors.Errorf("could not lock %s", s.lockPath)
	}
	defer func() {
		lock.Unlock()
		os.RemoveAll(s.lockPath)
	}()

	b, err := os.ReadFile(s.indexPath)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read %s", s.indexPath)
	}
	var idx ocispecs.Index
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, errors.Wrapf(err, "could not unmarshal %s (%q)", s.indexPath, string(b))
	}
	return &idx, nil
}

func (s StoreIndex) Put(tag string, desc ocispecs.Descriptor) error {
	// lock the store to prevent concurrent access
	lock := flock.New(s.lockPath)
	locked, err := lock.TryLock()
	if err != nil {
		return errors.Wrapf(err, "could not lock %s", s.lockPath)
	}
	if !locked {
		return errors.Errorf("could not lock %s", s.lockPath)
	}
	defer func() {
		lock.Unlock()
		os.RemoveAll(s.lockPath)
	}()

	// create the oci-layout file
	layout := ocispecs.ImageLayout{
		Version: ocispecs.ImageLayoutVersion,
	}
	layoutData, err := json.Marshal(layout)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.layoutPath, layoutData, 0644); err != nil {
		return err
	}

	// modify the index file
	idxFile, err := os.OpenFile(s.indexPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return errors.Wrapf(err, "could not open %s", s.indexPath)
	}
	defer idxFile.Close()

	var idx ocispecs.Index
	idxData, err := io.ReadAll(idxFile)
	if err != nil {
		return errors.Wrapf(err, "could not read %s", s.indexPath)
	}
	if len(idxData) > 0 {
		if err := json.Unmarshal(idxData, &idx); err != nil {
			return errors.Wrapf(err, "could not unmarshal %s (%q)", s.indexPath, string(idxData))
		}
	}

	if err = insertDesc(&idx, desc, tag); err != nil {
		return err
	}

	idxData, err = json.Marshal(idx)
	if err != nil {
		return err
	}
	if _, err = idxFile.WriteAt(idxData, 0); err != nil {
		return errors.Wrapf(err, "could not write %s", s.indexPath)
	}
	if err = idxFile.Truncate(int64(len(idxData))); err != nil {
		return errors.Wrapf(err, "could not truncate %s", s.indexPath)
	}
	return nil
}

func (s StoreIndex) Get(tag string) (*ocispecs.Descriptor, error) {
	idx, err := s.Read()
	if err != nil {
		return nil, err
	}

	for _, m := range idx.Manifests {
		if t, ok := m.Annotations[ocispecs.AnnotationRefName]; ok && t == tag {
			return &m, nil
		}
	}
	return nil, nil
}

func (s StoreIndex) GetSingle() (*ocispecs.Descriptor, error) {
	idx, err := s.Read()
	if err != nil {
		return nil, err
	}

	if len(idx.Manifests) == 1 {
		return &idx.Manifests[0], nil
	}
	return nil, nil
}

// insertDesc puts desc to index with tag.
// Existing manifests with the same tag will be removed from the index.
func insertDesc(index *ocispecs.Index, desc ocispecs.Descriptor, tag string) error {
	if index == nil {
		return nil
	}

	if index.SchemaVersion == 0 {
		index.SchemaVersion = 2
	}
	if tag != "" {
		if desc.Annotations == nil {
			desc.Annotations = make(map[string]string)
		}
		desc.Annotations[ocispecs.AnnotationRefName] = tag
		// remove existing manifests with the same tag
		var manifests []ocispecs.Descriptor
		for _, m := range index.Manifests {
			if m.Annotations[ocispecs.AnnotationRefName] != tag {
				manifests = append(manifests, m)
			}
		}
		index.Manifests = manifests
	}
	index.Manifests = append(index.Manifests, desc)
	return nil
}
