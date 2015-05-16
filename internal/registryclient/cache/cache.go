// Package cache provides facilities to speed up access to the storage
// backend.
package cache

import (
	"fmt"

	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
)

// BlobDescriptorCacheProvider provides repository scoped
// BlobDescriptorService cache instances and a global descriptor cache.
type BlobDescriptorCacheProvider interface {
	distribution.BlobDescriptorService

	RepositoryScoped(repo string) (distribution.BlobDescriptorService, error)
}

func validateDigest(dgst digest.Digest) error {
	return dgst.Validate()
}

func validateDescriptor(desc distribution.Descriptor) error {
	if err := validateDigest(desc.Digest); err != nil {
		return err
	}

	if desc.Length < 0 {
		return fmt.Errorf("cache: invalid length in descriptor: %v < 0", desc.Length)
	}

	if desc.MediaType == "" {
		return fmt.Errorf("cache: empty mediatype on descriptor: %v", desc)
	}

	return nil
}
