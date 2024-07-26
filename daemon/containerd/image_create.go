package containerd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/containerd/containerd/images"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/distribution/reference"
	"github.com/docker/docker/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageCreateFromJSON parses the JSON stream as an OCI index and creates an image from it.
// Currently, only OCI indexes are supported.
func (i *ImageService) ImageCreateFromJSON(ctx context.Context, ref reference.NamedTagged, jsonReader io.Reader) (ocispec.Descriptor, error) {
	var ociIndex ocispec.Index
	if err := json.NewDecoder(jsonReader).Decode(&ociIndex); err != nil {
		return ocispec.Descriptor{}, errdefs.InvalidParameter(fmt.Errorf("failed to decode JSON: %w", err))
	}

	if !containerdimages.IsIndexType(ociIndex.MediaType) {
		return ocispec.Descriptor{}, errdefs.InvalidParameter(errors.New("JSON is not an OCI index"))
	}

	if len(ociIndex.Manifests) == 0 {
		return ocispec.Descriptor{}, errdefs.InvalidParameter(errors.New("refusing to create an empty image"))
	}

	newImg := images.Image{
		Name:      ref.String(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	indexDesc, err := storeJson(ctx, i.content, ociIndex.MediaType, ociIndex, nil)
	if err != nil {
		return ocispec.Descriptor{}, errdefs.System(fmt.Errorf("failed to write modified image target: %w", err))
	}
	newImg.Target = indexDesc

	return indexDesc, i.forceCreateImage(ctx, newImg)
}
