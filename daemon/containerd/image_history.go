package containerd

import (
	"context"
	"encoding/json"

	"github.com/containerd/containerd/content"
	containerdimages "github.com/containerd/containerd/images"
	cplatforms "github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	imagetype "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/pkg/platforms"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ImageHistory returns a slice of HistoryResponseItem structures for the
// specified image name by walking the image lineage.
func (i *ImageService) ImageHistory(ctx context.Context, name string) ([]*imagetype.HistoryResponseItem, error) {
	desc, err := i.resolveDescriptor(ctx, name)
	if err != nil {
		return nil, err
	}

	cs := i.client.ContentStore()
	// TODO: pass the platform from the cli
	conf, err := containerdimages.Config(ctx, cs, desc, platforms.AllPlatformsWithPreference(cplatforms.Default()))
	if err != nil {
		return nil, err
	}

	diffIDs, err := containerdimages.RootFS(ctx, cs, conf)
	if err != nil {
		return nil, err
	}

	blob, err := content.ReadBlob(ctx, cs, conf)
	if err != nil {
		return nil, err
	}

	var image ocispec.Image
	if err := json.Unmarshal(blob, &image); err != nil {
		return nil, err
	}

	var (
		history []*imagetype.HistoryResponseItem
		sizes   []int64
	)
	s := i.client.SnapshotService(i.snapshotter)

	for i := range diffIDs {
		chainID := identity.ChainID(diffIDs[0 : i+1]).String()

		use, err := s.Usage(ctx, chainID)
		if err != nil {
			return nil, err
		}

		sizes = append(sizes, use.Size)
	}

	for _, h := range image.History {
		size := int64(0)
		if !h.EmptyLayer {
			if len(sizes) == 0 {
				return nil, errors.New("unable to find the size of the layer")
			}
			size = sizes[0]
			sizes = sizes[1:]
		}

		history = append([]*imagetype.HistoryResponseItem{{
			ID:        "<missing>",
			Comment:   h.Comment,
			CreatedBy: h.CreatedBy,
			Created:   h.Created.Unix(),
			Size:      size,
			Tags:      nil,
		}}, history...)
	}

	if len(history) != 0 {
		history[0].ID = desc.Digest.String()

		tagged, err := i.client.ImageService().List(ctx, "target.digest=="+desc.Digest.String())
		if err != nil {
			return nil, err
		}

		tags := make([]string, len(tagged))
		for i, t := range tagged {
			name, err := reference.ParseNamed(t.Name)
			if err != nil {
				return nil, err
			}
			tags[i] = reference.FamiliarString(name)
		}
		history[0].Tags = tags
	}

	return history, nil
}
