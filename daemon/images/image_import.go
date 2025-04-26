package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"encoding/json"
	"io"
	"time"

	c8dimages "github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/builder/dockerfile"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/moby/go-archive/compression"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImportImage imports an image, getting the archived layer data from layerReader.
// Uncompressed layer archive is passed to the layerStore and handled by the
// underlying graph driver.
// Image is tagged with the given reference.
// If the platform is nil, the default host platform is used.
// Message is used as the image's history comment.
// Image configuration is derived from the dockerfile instructions in changes.
func (i *ImageService) ImportImage(ctx context.Context, newRef reference.Named, platform *ocispec.Platform, msg string, layerReader io.Reader, changes []string) (ocispec.Descriptor, error) {
	if platform == nil {
		def := platforms.DefaultSpec()
		platform = &def
	}
	if err := image.CheckOS(platform.OS); err != nil {
		return ocispec.Descriptor{}, err
	}

	config, err := dockerfile.BuildFromConfig(ctx, &container.Config{}, changes, platform.OS)
	if err != nil {
		return ocispec.Descriptor{}, errdefs.InvalidParameter(err)
	}

	inflatedLayerData, err := compression.DecompressStream(layerReader)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	l, err := i.layerStore.Register(inflatedLayerData, "")
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer layer.ReleaseAndLog(i.layerStore, l)

	created := time.Now().UTC()
	imgConfig, err := json.Marshal(&image.Image{
		V1Image: image.V1Image{
			DockerVersion: dockerversion.Version,
			Config:        config,
			Architecture:  platform.Architecture,
			Variant:       platform.Variant,
			OS:            platform.OS,
			Created:       &created,
			Comment:       msg,
		},
		RootFS: &image.RootFS{
			Type:    "layers",
			DiffIDs: []layer.DiffID{l.DiffID()},
		},
		History: []image.History{{
			Created: &created,
			Comment: msg,
		}},
	})
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	id, err := i.imageStore.Create(imgConfig)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	desc := ocispec.Descriptor{
		MediaType: c8dimages.MediaTypeDockerSchema2Config,
		Digest:    id.Digest(),
		Size:      int64(len(imgConfig)),
	}

	if newRef != nil {
		if err := i.TagImage(ctx, desc, newRef); err != nil {
			return ocispec.Descriptor{}, err
		}
	}

	i.LogImageEvent(ctx, id.String(), id.String(), events.ActionImport)
	return desc, nil
}
