package daemon

import (
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/reference"
)

// ImageHistory returns a slice of ImageHistory structures for the specified image
// name by walking the image lineage.
func (daemon *Daemon) ImageHistory(name string) ([]*types.ImageHistory, error) {
	start := time.Now()
	img, err := daemon.GetImage(name)
	if err != nil {
		return nil, err
	}

	history := []*types.ImageHistory{}

	layerCounter := 0
	rootFS := *img.RootFS
	rootFS.DiffIDs = nil

	for _, h := range img.History {
		var layerSize int64

		if !h.EmptyLayer {
			if len(img.RootFS.DiffIDs) <= layerCounter {
				return nil, fmt.Errorf("too many non-empty layers in History section")
			}

			rootFS.Append(img.RootFS.DiffIDs[layerCounter])
			l, err := daemon.layerStore.Get(rootFS.ChainID())
			if err != nil {
				return nil, err
			}
			layerSize, err = l.DiffSize()
			layer.ReleaseAndLog(daemon.layerStore, l)
			if err != nil {
				return nil, err
			}

			layerCounter++
		}

		history = append([]*types.ImageHistory{{
			ID:        "<missing>",
			Created:   h.Created.Unix(),
			CreatedBy: h.CreatedBy,
			Comment:   h.Comment,
			Size:      layerSize,
		}}, history...)
	}

	// Fill in image IDs and tags
	histImg := img
	id := img.ID()
	for _, h := range history {
		h.ID = id.String()

		var tags []string
		for _, r := range daemon.referenceStore.References(id.Digest()) {
			if _, ok := r.(reference.NamedTagged); ok {
				tags = append(tags, r.String())
			}
		}

		h.Tags = tags

		id = histImg.Parent
		if id == "" {
			break
		}
		histImg, err = daemon.GetImage(id.String())
		if err != nil {
			break
		}
	}
	imageActions.WithValues("history").UpdateSince(start)
	return history, nil
}
