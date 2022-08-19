package containerd

import (
	"context"
	"encoding/json"

	"github.com/containerd/containerd/content"
	imagetype "github.com/docker/docker/api/types/image"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ImageHistory returns a slice of ImageHistory structures for the specified
// image name by walking the image lineage.
func (i *ImageService) ImageHistory(ctx context.Context, name string) ([]*imagetype.HistoryResponseItem, error) {
	c8dimg, _, err := i.getImage(ctx, name)
	if err != nil {
		return nil, err
	}

	config, err := c8dimg.Config(ctx)
	if err != nil {
		return nil, err
	}

	blob, err := content.ReadBlob(ctx, c8dimg.ContentStore(), config)
	if err != nil {
		return nil, err
	}

	var image ocispec.Image
	if err := json.Unmarshal(blob, &image); err != nil {
		return nil, err
	}

	history := []*imagetype.HistoryResponseItem{}

	diffIDs, err := c8dimg.RootFS(ctx)
	if err != nil {
		return nil, err
	}

	sizes := []int64{}
	s := i.client.SnapshotService(i.snapshotter)
	for i := range diffIDs {
		diffIDs := diffIDs[0 : i+1]
		chainID := identity.ChainID(diffIDs).String()

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
		}}, history...)
	}

	if len(history) != 0 {
		history[0].ID = c8dimg.Target().Digest.String()
	}

	return history, nil
}
