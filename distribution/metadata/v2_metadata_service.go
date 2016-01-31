package metadata

import (
	"encoding/json"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/layer"
)

// V2MetadataService maps layer IDs to a set of known metadata for
// the layer.
type V2MetadataService struct {
	store Store
}

// V2Metadata contains the digest and source repository information for a layer.
type V2Metadata struct {
	Digest           digest.Digest
	SourceRepository string
}

// maxMetadata is the number of metadata entries to keep per layer DiffID.
const maxMetadata = 50

// NewV2MetadataService creates a new diff ID to v2 metadata mapping service.
func NewV2MetadataService(store Store) *V2MetadataService {
	return &V2MetadataService{
		store: store,
	}
}

func (serv *V2MetadataService) diffIDNamespace() string {
	return "v2metadata-by-diffid"
}

func (serv *V2MetadataService) digestNamespace() string {
	return "diffid-by-digest"
}

func (serv *V2MetadataService) diffIDKey(diffID layer.DiffID) string {
	return string(digest.Digest(diffID).Algorithm()) + "/" + digest.Digest(diffID).Hex()
}

func (serv *V2MetadataService) digestKey(dgst digest.Digest) string {
	return string(dgst.Algorithm()) + "/" + dgst.Hex()
}

// GetMetadata finds the metadata associated with a layer DiffID.
func (serv *V2MetadataService) GetMetadata(diffID layer.DiffID) ([]V2Metadata, error) {
	jsonBytes, err := serv.store.Get(serv.diffIDNamespace(), serv.diffIDKey(diffID))
	if err != nil {
		return nil, err
	}

	var metadata []V2Metadata
	if err := json.Unmarshal(jsonBytes, &metadata); err != nil {
		return nil, err
	}

	return metadata, nil
}

// GetDiffID finds a layer DiffID from a digest.
func (serv *V2MetadataService) GetDiffID(dgst digest.Digest) (layer.DiffID, error) {
	diffIDBytes, err := serv.store.Get(serv.digestNamespace(), serv.digestKey(dgst))
	if err != nil {
		return layer.DiffID(""), err
	}

	return layer.DiffID(diffIDBytes), nil
}

// Add associates metadata with a layer DiffID. If too many metadata entries are
// present, the oldest one is dropped.
func (serv *V2MetadataService) Add(diffID layer.DiffID, metadata V2Metadata) error {
	oldMetadata, err := serv.GetMetadata(diffID)
	if err != nil {
		oldMetadata = nil
	}
	newMetadata := make([]V2Metadata, 0, len(oldMetadata)+1)

	// Copy all other metadata to new slice
	for _, oldMeta := range oldMetadata {
		if oldMeta != metadata {
			newMetadata = append(newMetadata, oldMeta)
		}
	}

	newMetadata = append(newMetadata, metadata)

	if len(newMetadata) > maxMetadata {
		newMetadata = newMetadata[len(newMetadata)-maxMetadata:]
	}

	jsonBytes, err := json.Marshal(newMetadata)
	if err != nil {
		return err
	}

	err = serv.store.Set(serv.diffIDNamespace(), serv.diffIDKey(diffID), jsonBytes)
	if err != nil {
		return err
	}

	return serv.store.Set(serv.digestNamespace(), serv.digestKey(metadata.Digest), []byte(diffID))
}

// Remove unassociates a metadata entry from a layer DiffID.
func (serv *V2MetadataService) Remove(metadata V2Metadata) error {
	diffID, err := serv.GetDiffID(metadata.Digest)
	if err != nil {
		return err
	}
	oldMetadata, err := serv.GetMetadata(diffID)
	if err != nil {
		oldMetadata = nil
	}
	newMetadata := make([]V2Metadata, 0, len(oldMetadata))

	// Copy all other metadata to new slice
	for _, oldMeta := range oldMetadata {
		if oldMeta != metadata {
			newMetadata = append(newMetadata, oldMeta)
		}
	}

	if len(newMetadata) == 0 {
		return serv.store.Delete(serv.diffIDNamespace(), serv.diffIDKey(diffID))
	}

	jsonBytes, err := json.Marshal(newMetadata)
	if err != nil {
		return err
	}

	return serv.store.Set(serv.diffIDNamespace(), serv.diffIDKey(diffID), jsonBytes)
}
