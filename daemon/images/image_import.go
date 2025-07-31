package images

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/moby/go-archive/compression"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/v2/daemon/builder/dockerfile"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/internal/layer"
	"github.com/moby/moby/v2/dockerversion"
	"github.com/moby/moby/v2/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImportImage imports an image, getting the archived layer data from layerReader.
// Uncompressed layer archive is passed to the layerStore and handled by the
// underlying graph driver.
// Image is tagged with the given reference.
// If the platform is nil, the default host platform is used.
// Message is used as the image's history comment.
// Image configuration is derived from the dockerfile instructions in changes.
func (i *ImageService) ImportImage(ctx context.Context, newRef reference.Named, platform *ocispec.Platform, msg string, layerReader io.Reader, changes []string) (image.ID, error) {
	if platform == nil {
		def := platforms.DefaultSpec()
		platform = &def
	}
	if err := image.CheckOS(platform.OS); err != nil {
		return "", err
	}

	config, err := dockerfile.BuildFromConfig(ctx, &container.Config{}, changes, platform.OS)
	if err != nil {
		return "", errdefs.InvalidParameter(err)
	}

	inflatedLayerData, err := compression.DecompressStream(layerReader)
	if err != nil {
		return "", err
	}
	l, err := i.layerStore.Register(inflatedLayerData, "")
	if err != nil {
		return "", err
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
		return "", err
	}

	id, err := i.imageStore.Create(imgConfig)
	if err != nil {
		return "", err
	}

	if newRef != nil {
		if err := i.TagImage(ctx, id, newRef); err != nil {
			return "", err
		}
	}

	i.LogImageEvent(ctx, id.String(), id.String(), events.ActionImport)
	return id, nil
}
