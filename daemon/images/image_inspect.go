package images

import (
	"context"
	"time"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/backend"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/storage"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
)

func (i *ImageService) ImageInspect(ctx context.Context, refOrID string, _ backend.ImageInspectOpts) (*imagetypes.InspectResponse, error) {
	img, err := i.GetImage(ctx, refOrID, backend.GetImageOpts{})
	if err != nil {
		return nil, err
	}

	size, layerMetadata, err := i.getLayerSizeAndMetadata(img)
	if err != nil {
		return nil, err
	}

	lastUpdated, err := i.imageStore.GetLastUpdated(img.ID())
	if err != nil {
		return nil, err
	}

	var repoTags, repoDigests []string
	for _, ref := range i.referenceStore.References(img.ID().Digest()) {
		switch ref.(type) {
		case reference.NamedTagged:
			repoTags = append(repoTags, reference.FamiliarString(ref))
		case reference.Canonical:
			repoDigests = append(repoDigests, reference.FamiliarString(ref))
		}
	}

	comment := img.Comment
	if len(comment) == 0 && len(img.History) > 0 {
		comment = img.History[len(img.History)-1].Comment
	}

	var created string
	if img.Created != nil {
		created = img.Created.Format(time.RFC3339Nano)
	}

	var layers []string
	for _, l := range img.RootFS.DiffIDs {
		layers = append(layers, l.String())
	}

	return &imagetypes.InspectResponse{
		ID:              img.ID().String(),
		RepoTags:        repoTags,
		RepoDigests:     repoDigests,
		Parent:          img.Parent.String(),
		Comment:         comment,
		Created:         created,
		Container:       img.Container,        //nolint:staticcheck // ignore SA1019: field is deprecated, but still set on API < v1.45.
		ContainerConfig: &img.ContainerConfig, //nolint:staticcheck // ignore SA1019: field is deprecated, but still set on API < v1.45.
		DockerVersion:   img.DockerVersion,
		Author:          img.Author,
		Config:          img.Config,
		Architecture:    img.Architecture,
		Variant:         img.Variant,
		Os:              img.OperatingSystem(),
		OsVersion:       img.OSVersion,
		Size:            size,
		GraphDriver: storage.DriverData{
			Name: i.layerStore.DriverName(),
			Data: layerMetadata,
		},
		RootFS: imagetypes.RootFS{
			Type:   img.RootFS.Type,
			Layers: layers,
		},
		Metadata: imagetypes.Metadata{
			LastTagTime: lastUpdated,
		},
	}, nil
}

func (i *ImageService) getLayerSizeAndMetadata(img *image.Image) (int64, map[string]string, error) {
	var size int64
	var layerMetadata map[string]string
	layerID := img.RootFS.ChainID()
	if layerID != "" {
		l, err := i.layerStore.Get(layerID)
		if err != nil {
			return 0, nil, err
		}
		defer layer.ReleaseAndLog(i.layerStore, l)
		size = l.Size()
		layerMetadata, err = l.Metadata()
		if err != nil {
			return 0, nil, err
		}
	}
	return size, layerMetadata, nil
}
