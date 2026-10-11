package specialimage

import (
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ForeignLayer builds a single-platform image that has a single
// non-distributable (a.k.a. foreign) layer.
//
// The layer blob is written to the blobs directory so that the image can be
// pushed to a registry and pulled, but its manifest descriptor carries a
// non-distributable media type (ocispec.MediaTypeImageLayerNonDistributable).
// Consumers such as `docker save` treat non-distributable layers specially:
// they are never fetched to satisfy missing content and are excluded from the
// exported archive (see archive.WithSkipNonDistributableBlobs).
func ForeignLayer(dir string) (*ocispec.Index, error) {
	const imageRef = "foreign:latest"
	ref, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return nil, err
	}

	layerDesc, err := writeLayerWithOneFile(dir, "foreign", []byte("foreign-layer-content"))
	if err != nil {
		return nil, err
	}
	// Mark the layer as non-distributable. The blob on disk is identical to a
	// regular layer; only the media type (recorded in the manifest) signals that
	// it should not be redistributed. No URLs are set so that a registry fetch
	// falls back to the standard blobs endpoint.
	layerDesc.MediaType = ocispec.MediaTypeImageLayerNonDistributable

	img := ocispec.Image{
		Platform: platforms.DefaultSpec(),
		Config: ocispec.ImageConfig{
			Env: []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		},
		RootFS: ocispec.RootFS{
			Type:    "layers",
			DiffIDs: []digest.Digest{layerDesc.Digest},
		},
	}
	configDesc, err := writeJsonBlob(dir, ocispec.MediaTypeImageConfig, img)
	if err != nil {
		return nil, err
	}

	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    []ocispec.Descriptor{layerDesc},
	}

	legacyManifests := []manifestItem{{
		Config:   blobPath(configDesc),
		RepoTags: []string{imageRef},
		Layers:   []string{blobPath(layerDesc)},
	}}

	return singlePlatformImage(dir, ref, manifest, legacyManifests)
}
