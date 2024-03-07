package specialimage

import (
	"os"
	"path/filepath"

	"github.com/containerd/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TwoPlatform(dir string) (*ocispec.Index, error) {
	const imageRef = "twoplatform:latest"

	layer1Desc, err := writeLayerWithOneFile(dir, "bash", []byte("layer1"))
	if err != nil {
		return nil, err
	}
	layer2Desc, err := writeLayerWithOneFile(dir, "bash", []byte("layer2"))
	if err != nil {
		return nil, err
	}

	config1Desc, err := writeJsonBlob(dir, ocispec.MediaTypeImageConfig, ocispec.Image{
		Platform: platforms.MustParse("linux/amd64"),
		Config: ocispec.ImageConfig{
			Env: []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		},
		RootFS: ocispec.RootFS{
			Type:    "layers",
			DiffIDs: []digest.Digest{layer1Desc.Digest},
		},
	})
	if err != nil {
		return nil, err
	}

	manifest1Desc, err := writeJsonBlob(dir, ocispec.MediaTypeImageManifest, ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    config1Desc,
		Layers:    []ocispec.Descriptor{layer1Desc},
	})
	if err != nil {
		return nil, err
	}

	config2Desc, err := writeJsonBlob(dir, ocispec.MediaTypeImageConfig, ocispec.Image{
		Platform: platforms.MustParse("linux/arm64"),
		Config: ocispec.ImageConfig{
			Env: []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		},
		RootFS: ocispec.RootFS{
			Type:    "layers",
			DiffIDs: []digest.Digest{layer1Desc.Digest},
		},
	})
	if err != nil {
		return nil, err
	}

	manifest2Desc, err := writeJsonBlob(dir, ocispec.MediaTypeImageManifest, ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    config2Desc,
		Layers:    []ocispec.Descriptor{layer2Desc},
	})
	if err != nil {
		return nil, err
	}

	index := ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{manifest1Desc, manifest2Desc},
	}

	ref, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return nil, err
	}
	return multiPlatformImage(dir, ref, index)
}

func multiPlatformImage(dir string, ref reference.Named, target ocispec.Index) (*ocispec.Index, error) {
	targetDesc, err := writeJsonBlob(dir, ocispec.MediaTypeImageIndex, target)
	if err != nil {
		return nil, err
	}

	if ref != nil {
		targetDesc.Annotations = map[string]string{
			"io.containerd.image.name": ref.String(),
		}

		if tagged, ok := ref.(reference.Tagged); ok {
			targetDesc.Annotations[ocispec.AnnotationRefName] = tagged.Tag()
		}
	}

	index := ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{targetDesc},
	}

	if err := writeJson(index, filepath.Join(dir, "index.json")); err != nil {
		return nil, err
	}

	err = os.WriteFile(filepath.Join(dir, "oci-layout"), []byte(`{"imageLayoutVersion": "1.0.0"}`), 0o644)
	if err != nil {
		return nil, err
	}

	return &index, nil
}
