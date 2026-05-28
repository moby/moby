package specialimage

import (
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Labeled creates a minimal image with the given labels set in its config.
// Suitable for testing label-based filters without depending on layer content.
func Labeled(dir string, imageRef string, labels map[string]string) (*ocispec.Index, error) {
	ref, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return nil, err
	}

	configDesc, err := writeJsonBlob(dir, ocispec.MediaTypeImageConfig, ocispec.Image{
		Platform: platforms.DefaultSpec(),
		Config: ocispec.ImageConfig{
			Labels: labels,
		},
		RootFS: ocispec.RootFS{
			Type: "layers",
		},
	})
	if err != nil {
		return nil, err
	}

	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    []ocispec.Descriptor{},
	}

	legacyManifests := []manifestItem{
		{
			Config:   blobPath(configDesc),
			RepoTags: []string{imageRef},
		},
	}

	return singlePlatformImage(dir, ref, manifest, legacyManifests)
}
