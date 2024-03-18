package specialimage

import (
	"github.com/distribution/reference"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// EmptyIndex creates an image index with no manifests.
// This is equivalent to `tianon/scratch:index`.
func EmptyIndex(dir string) (*ocispec.Index, error) {
	const imageRef = "emptyindex:latest"

	index := ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{},
	}

	ref, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return nil, err
	}
	return multiPlatformImage(dir, ref, index)
}
