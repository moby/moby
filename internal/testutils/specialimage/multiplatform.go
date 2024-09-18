package specialimage

import (
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func MultiPlatform(dir string, imageRef string, imagePlatforms []ocispec.Platform) (*ocispec.Index, []ocispec.Descriptor, error) {
	ref, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return nil, nil, err
	}

	var descs []ocispec.Descriptor

	for _, platform := range imagePlatforms {
		ps := platforms.Format(platform)
		manifestDesc, err := oneLayerPlatformManifest(dir, platform, FileInLayer{Path: "bash", Content: []byte("layer-" + ps)})
		if err != nil {
			return nil, nil, err
		}
		descs = append(descs, manifestDesc)
	}

	idx, err := multiPlatformImage(dir, ref, ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: descs,
	})
	return idx, descs, err
}
