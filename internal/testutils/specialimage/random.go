package specialimage

import (
	"math/rand"
	"strconv"

	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func RandomSinglePlatform(dir string, platform ocispec.Platform, source rand.Source) (*ocispec.Index, error) {
	r := rand.New(source) //nolint:gosec // Ignore G404: Use of weak random number generator (math/rand instead of crypto/rand)

	imageRef := "random-" + strconv.FormatInt(r.Int63(), 10) + ":latest"

	layerCount := r.Intn(8)

	var layers []ocispec.Descriptor
	for i := 0; i < layerCount; i++ {
		layerDesc, err := writeLayerWithOneFile(dir, "layer-"+strconv.Itoa(i), []byte(strconv.Itoa(i)))
		if err != nil {
			return nil, err
		}
		layers = append(layers, layerDesc)
	}

	configDesc, err := writeJsonBlob(dir, ocispec.MediaTypeImageConfig, ocispec.Image{
		Platform: platform,
		Config: ocispec.ImageConfig{
			Env: []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		},
		RootFS: ocispec.RootFS{
			Type:    "layers",
			DiffIDs: layersToDigests(layers),
		},
	})
	if err != nil {
		return nil, err
	}

	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    layers,
	}

	legacyManifests := []manifestItem{
		{
			Config:   blobPath(configDesc),
			RepoTags: []string{imageRef},
			Layers:   blobPaths(layers),
		},
	}

	ref, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return nil, err
	}
	return singlePlatformImage(dir, ref, manifest, legacyManifests)
}

func layersToDigests(layers []ocispec.Descriptor) []digest.Digest {
	var digests []digest.Digest
	for _, l := range layers {
		digests = append(digests, l.Digest)
	}
	return digests
}

func blobPaths(descriptors []ocispec.Descriptor) []string {
	var paths []string
	for _, d := range descriptors {
		paths = append(paths, blobPath(d))
	}
	return paths
}
