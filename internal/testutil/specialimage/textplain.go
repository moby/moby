package specialimage

import (
	"strings"

	"github.com/distribution/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// TextPlain creates an non-container image that only contains a text/plain blob.
func TextPlain(dir string) (*ocispec.Index, error) {
	ref, err := reference.ParseNormalizedNamed("tianon/test:text-plain")
	if err != nil {
		return nil, err
	}

	emptyJsonDesc, err := writeBlob(dir, "text/plain", strings.NewReader("{}"))
	if err != nil {
		return nil, err
	}

	configDesc := emptyJsonDesc
	configDesc.MediaType = "application/vnd.oci.empty.v1+json"

	desc, err := writeJsonBlob(dir, ocispec.MediaTypeImageManifest, ocispec.Manifest{
		Config: configDesc,
		Layers: []ocispec.Descriptor{
			emptyJsonDesc,
		},
	})
	if err != nil {
		return nil, err
	}
	desc.Annotations = map[string]string{
		"io.containerd.image.name": ref.String(),
	}

	return ociImage(dir, nil, desc)
}
