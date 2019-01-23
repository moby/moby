package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/layer"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ImageHistory returns a slice of ImageHistory structures for the specified image
// name by walking the image lineage.
func (i *ImageService) ImageHistory(ctx context.Context, name string) ([]*image.HistoryResponseItem, error) {
	start := time.Now()
	ci, err := i.getCachedRef(ctx, name)
	if err != nil {
		return nil, err
	}

	p, err := content.ReadBlob(ctx, i.client.ContentStore(), ci.config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config")
	}

	var img ocispec.Image
	if err := json.Unmarshal(p, &img); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config")
	}

	history := []*image.HistoryResponseItem{}

	layerCounter := 0
	rootFS := img.RootFS
	rootFS.DiffIDs = nil

	for _, h := range img.History {
		var layerSize int64

		if !h.EmptyLayer {
			if len(img.RootFS.DiffIDs) <= layerCounter {
				return nil, fmt.Errorf("too many non-empty layers in History section")
			}
			rootFS.DiffIDs = append(rootFS.DiffIDs, img.RootFS.DiffIDs[layerCounter])
			l, err := i.layerStores[runtime.GOOS].Get(layer.ChainID(identity.ChainID(rootFS.DiffIDs)))
			if err != nil {
				return nil, err
			}
			layerSize, err = l.DiffSize()
			layer.ReleaseAndLog(i.layerStores[runtime.GOOS], l)
			if err != nil {
				return nil, err
			}

			layerCounter++
		}

		history = append([]*image.HistoryResponseItem{{
			ID:        "<missing>",
			Created:   h.Created.Unix(),
			CreatedBy: h.CreatedBy,
			Comment:   h.Comment,
			Size:      layerSize,
		}}, history...)
	}

	c, err := i.getCache(ctx)
	if err != nil {
		return nil, err
	}

	// Fill in image IDs and tags
	histImg := ci
	id := ci.config.Digest
	for _, h := range history {
		h.ID = id.String()

		var tags []string
		for _, r := range histImg.references {
			if _, ok := r.(reference.NamedTagged); ok {
				tags = append(tags, reference.FamiliarString(r))
			}
		}

		h.Tags = tags

		id = histImg.parent
		if id == "" {
			break
		}
		histImg = c.byID(id)
		if histImg == nil {
			break
		}
	}
	imageActions.WithValues("history").UpdateSince(start)
	return history, nil
}
