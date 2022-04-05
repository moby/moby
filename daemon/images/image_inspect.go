package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/pkg/errors"
)

// LookupImage looks up an image by name and returns it as an ImageInspect
// structure.
func (i *ImageService) LookupImage(ctx context.Context, name string) (*types.ImageInspect, error) {
	img, err := i.GetImage(ctx, name, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "no such image: %s", name)
	}

	refs := i.referenceStore.References(img.ID().Digest())
	repoTags := []string{}
	repoDigests := []string{}
	for _, ref := range refs {
		switch ref.(type) {
		case reference.NamedTagged:
			repoTags = append(repoTags, reference.FamiliarString(ref))
		case reference.Canonical:
			repoDigests = append(repoDigests, reference.FamiliarString(ref))
		}
	}

	var size int64
	var layerMetadata map[string]string
	layerID := img.RootFS.ChainID()
	if layerID != "" {
		l, err := i.layerStore.Get(layerID)
		if err != nil {
			return nil, err
		}
		defer layer.ReleaseAndLog(i.layerStore, l)
		size = l.Size()
		layerMetadata, err = l.Metadata()
		if err != nil {
			return nil, err
		}
	}

	comment := img.Comment
	if len(comment) == 0 && len(img.History) > 0 {
		comment = img.History[len(img.History)-1].Comment
	}

	lastUpdated, err := i.imageStore.GetLastUpdated(ctx, img.ID())
	if err != nil {
		return nil, err
	}

	return &types.ImageInspect{
		ID:              img.ID().String(),
		RepoTags:        repoTags,
		RepoDigests:     repoDigests,
		Parent:          img.Parent.String(),
		Comment:         comment,
		Created:         img.Created.Format(time.RFC3339Nano),
		Container:       img.Container,
		ContainerConfig: &img.ContainerConfig,
		DockerVersion:   img.DockerVersion,
		Author:          img.Author,
		Config:          img.Config,
		Architecture:    img.Architecture,
		Variant:         img.Variant,
		Os:              img.OperatingSystem(),
		OsVersion:       img.OSVersion,
		Size:            size,
		VirtualSize:     size, // TODO: field unused, deprecate
		GraphDriver: types.GraphDriverData{
			Name: i.layerStore.DriverName(),
			Data: layerMetadata,
		},
		RootFS: rootFSToAPIType(img.RootFS),
		Metadata: types.ImageMetadata{
			LastTagTime: lastUpdated,
		},
	}, nil
}

func rootFSToAPIType(rootfs *image.RootFS) types.RootFS {
	var layers []string
	for _, l := range rootfs.DiffIDs {
		layers = append(layers, l.String())
	}
	return types.RootFS{
		Type:   rootfs.Type,
		Layers: layers,
	}
}
