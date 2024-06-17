package specialimage

import (
	"github.com/containerd/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func MultiPlatform(dir string, imageRef string, imagePlatforms []ocispec.Platform) (*ocispec.Index, error) {
	ref, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return nil, err
	}

	var descs []ocispec.Descriptor

	for _, platform := range imagePlatforms {
		ps := platforms.Format(platform)
		manifestDesc, err := oneLayerPlatformManifest(dir, platform, FileInLayer{Path: "bash", Content: []byte("layer-" + ps)})
		if err != nil {
			return nil, err
		}
		descs = append(descs, manifestDesc)
	}

	return multiPlatformImage(dir, ref, ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: descs,
	})
}
