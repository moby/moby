package ociindex

import (
	"encoding/json"
	"io"
	"maps"
	"os"
	"path"
	"syscall"

	"github.com/containerd/containerd/v2/pkg/reference"
	"github.com/gofrs/flock"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	// lockFileSuffix is the suffix of the lock file
	lockFileSuffix = ".lock"

	annotationImageName = "io.containerd.image.name"
)

type StoreIndex struct {
	indexPath  string
	lockPath   string
	layoutPath string
}

type NameOrTag struct {
	isTag bool
	value string
}

func Name(name string) NameOrTag {
	return NameOrTag{value: name}
}

func Tag(tag string) NameOrTag {
	return NameOrTag{isTag: true, value: tag}
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
		if !errors.Is(err, syscall.EPERM) && !errors.Is(err, syscall.EROFS) {
			return nil, errors.Wrapf(err, "could not lock %s", s.lockPath)
		}
	} else {
		if !locked {
			return nil, errors.Errorf("could not lock %s", s.lockPath)
		}
		defer func() {
			lock.Unlock()
			os.RemoveAll(s.lockPath)
		}()
	}

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

func (s StoreIndex) Put(desc ocispecs.Descriptor, names ...NameOrTag) error {
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

	setOCIIndexDefaults(&idx)

	namesp := make([]*NameOrTag, 0, len(names))
	for _, n := range names {
		namesp = append(namesp, &n)
	}
	if len(names) == 0 {
		namesp = append(namesp, nil)
	}

	for _, name := range namesp {
		if err = insertDesc(&idx, desc, name); err != nil {
			return err
		}
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
		if t, ok := m.Annotations[annotationImageName]; ok && t == tag {
			return &m, nil
		}
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

// setOCIIndexDefaults updates zero values in index to their default values.
func setOCIIndexDefaults(index *ocispecs.Index) {
	if index == nil {
		return
	}
	if index.SchemaVersion == 0 {
		index.SchemaVersion = 2
	}
	if index.MediaType == "" {
		index.MediaType = ocispecs.MediaTypeImageIndex
	}
}

// insertDesc puts desc to index with tag.
// Existing manifests with the same tag will be removed from the index.
func insertDesc(index *ocispecs.Index, in ocispecs.Descriptor, name *NameOrTag) error {
	if index == nil {
		return nil
	}

	// make a copy to not modify the input descriptor
	desc := in
	desc.Annotations = maps.Clone(in.Annotations)

	if name != nil {
		if desc.Annotations == nil {
			desc.Annotations = make(map[string]string)
		}
		imgName, refName := name.value, name.value
		if name.isTag {
			imgName = ""
		} else {
			refName = ociReferenceName(imgName)
		}

		if imgName != "" {
			desc.Annotations[annotationImageName] = imgName
		}
		desc.Annotations[ocispecs.AnnotationRefName] = refName
		// remove existing manifests with the same tag/name
		var manifests []ocispecs.Descriptor
		for _, m := range index.Manifests {
			if m.Annotations[ocispecs.AnnotationRefName] != refName || m.Annotations[annotationImageName] != imgName {
				manifests = append(manifests, m)
			}
		}
		index.Manifests = manifests
	}
	index.Manifests = append(index.Manifests, desc)
	return nil
}

// ociReferenceName takes the loosely defined reference name same way as
// containerd tar exporter does.
func ociReferenceName(name string) string {
	// OCI defines the reference name as only a tag excluding the
	// repository. The containerd annotation contains the full image name
	// since the tag is insufficient for correctly naming and referring to an
	// image
	var ociRef string
	if spec, err := reference.Parse(name); err == nil {
		ociRef = spec.Object
	} else {
		ociRef = name
	}

	return ociRef
}
