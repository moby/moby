package specialimage

import (
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type PartialOpts struct {
	Stored  []ocispec.Platform
	Missing []ocispec.Platform
}

// PartialMultiPlatform creates an index with all platforms in storedPlatforms
// and missingPlatforms. However, only the blobs of the storedPlatforms are
// created and stored, while the missingPlatforms are only referenced in the
// index.
func PartialMultiPlatform(dir string, imageRef string, opts PartialOpts) (*ocispec.Index, []ocispec.Descriptor, error) {
	ref, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return nil, nil, err
	}

	var descs []ocispec.Descriptor

	for _, platform := range opts.Stored {
		ps := platforms.Format(platform)
		manifestDesc, err := oneLayerPlatformManifest(dir, platform, FileInLayer{Path: "bash", Content: []byte("layer-" + ps)})
		if err != nil {
			return nil, nil, err
		}
		descs = append(descs, manifestDesc)
	}

	for _, platform := range opts.Missing {
		platformStr := platforms.FormatAll(platform)
		dgst := digest.FromBytes([]byte(platformStr))

		descs = append(descs, ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageManifest,
			Size:      128,
			Platform:  &platform,
			Digest:    dgst,
		})
	}

	idx, err := multiPlatformImage(dir, ref, ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: descs,
	})
	return idx, descs, err
}
