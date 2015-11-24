package metadata

import (
	"encoding/json"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/layer"
)

// BlobSumService maps layer IDs to a set of known blobsums for
// the layer.
type BlobSumService struct {
	store Store
}

// maxBlobSums is the number of blobsums to keep per layer DiffID.
const maxBlobSums = 5

// NewBlobSumService creates a new blobsum mapping service.
func NewBlobSumService(store Store) *BlobSumService {
	return &BlobSumService{
		store: store,
	}
}

func (blobserv *BlobSumService) diffIDNamespace() string {
	return "blobsum-storage"
}

func (blobserv *BlobSumService) blobSumNamespace() string {
	return "blobsum-lookup"
}

func (blobserv *BlobSumService) diffIDKey(diffID layer.DiffID) string {
	return string(digest.Digest(diffID).Algorithm()) + "/" + digest.Digest(diffID).Hex()
}

func (blobserv *BlobSumService) blobSumKey(blobsum digest.Digest) string {
	return string(blobsum.Algorithm()) + "/" + blobsum.Hex()
}

// GetBlobSums finds the blobsums associated with a layer DiffID.
func (blobserv *BlobSumService) GetBlobSums(diffID layer.DiffID) ([]digest.Digest, error) {
	jsonBytes, err := blobserv.store.Get(blobserv.diffIDNamespace(), blobserv.diffIDKey(diffID))
	if err != nil {
		return nil, err
	}

	var blobsums []digest.Digest
	if err := json.Unmarshal(jsonBytes, &blobsums); err != nil {
		return nil, err
	}

	return blobsums, nil
}

// GetDiffID finds a layer DiffID from a blobsum hash.
func (blobserv *BlobSumService) GetDiffID(blobsum digest.Digest) (layer.DiffID, error) {
	diffIDBytes, err := blobserv.store.Get(blobserv.blobSumNamespace(), blobserv.blobSumKey(blobsum))
	if err != nil {
		return layer.DiffID(""), err
	}

	return layer.DiffID(diffIDBytes), nil
}

// Add associates a blobsum with a layer DiffID. If too many blobsums are
// present, the oldest one is dropped.
func (blobserv *BlobSumService) Add(diffID layer.DiffID, blobsum digest.Digest) error {
	oldBlobSums, err := blobserv.GetBlobSums(diffID)
	if err != nil {
		oldBlobSums = nil
	}
	newBlobSums := make([]digest.Digest, 0, len(oldBlobSums)+1)

	// Copy all other blobsums to new slice
	for _, oldSum := range oldBlobSums {
		if oldSum != blobsum {
			newBlobSums = append(newBlobSums, oldSum)
		}
	}

	newBlobSums = append(newBlobSums, blobsum)

	if len(newBlobSums) > maxBlobSums {
		newBlobSums = newBlobSums[len(newBlobSums)-maxBlobSums:]
	}

	jsonBytes, err := json.Marshal(newBlobSums)
	if err != nil {
		return err
	}

	err = blobserv.store.Set(blobserv.diffIDNamespace(), blobserv.diffIDKey(diffID), jsonBytes)
	if err != nil {
		return err
	}

	return blobserv.store.Set(blobserv.blobSumNamespace(), blobserv.blobSumKey(blobsum), []byte(diffID))
}
