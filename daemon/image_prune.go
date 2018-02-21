package daemon

import (
	"sync/atomic"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

var imagesAcceptedFilters = map[string]bool{
	"dangling": true,
	"label":    true,
	"label!":   true,
	"until":    true,
}

// ImagesPrune removes unused images
func (daemon *Daemon) ImagesPrune(ctx context.Context, pruneFilters filters.Args) (*types.ImagesPruneReport, error) {
	if !atomic.CompareAndSwapInt32(&daemon.pruneRunning, 0, 1) {
		return nil, errPruneRunning
	}
	defer atomic.StoreInt32(&daemon.pruneRunning, 0)

	// make sure that only accepted filters have been received
	err := pruneFilters.Validate(imagesAcceptedFilters)
	if err != nil {
		return nil, err
	}

	rep := &types.ImagesPruneReport{}

	danglingOnly := true
	if pruneFilters.Contains("dangling") {
		if pruneFilters.ExactMatch("dangling", "false") || pruneFilters.ExactMatch("dangling", "0") {
			danglingOnly = false
		} else if !pruneFilters.ExactMatch("dangling", "true") && !pruneFilters.ExactMatch("dangling", "1") {
			return nil, invalidFilter{"dangling", pruneFilters.Get("dangling")}
		}
	}

	until, err := getUntilFromPruneFilters(pruneFilters)
	if err != nil {
		return nil, err
	}

	var allImages map[image.ID]*image.Image
	if danglingOnly {
		allImages = daemon.imageStore.Heads()
	} else {
		allImages = daemon.imageStore.Map()
	}

	// Filter intermediary images and get their unique size
	allLayers := make(map[layer.ChainID]layer.Layer)
	for _, ls := range daemon.layerStores {
		for k, v := range ls.Map() {
			allLayers[k] = v
		}
	}
	topImages := map[image.ID]*image.Image{}
	for id, img := range allImages {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			dgst := digest.Digest(id)
			if len(daemon.referenceStore.References(dgst)) == 0 && len(daemon.imageStore.Children(id)) != 0 {
				continue
			}
			if !until.IsZero() && img.Created.After(until) {
				continue
			}
			if img.Config != nil && !matchLabels(pruneFilters, img.Config.Labels) {
				continue
			}
			topImages[id] = img
		}
	}

	canceled := false
deleteImagesLoop:
	for id := range topImages {
		select {
		case <-ctx.Done():
			// we still want to calculate freed size and return the data
			canceled = true
			break deleteImagesLoop
		default:
		}

		deletedImages := []types.ImageDeleteResponseItem{}
		refs := daemon.referenceStore.References(id.Digest())
		if len(refs) > 0 {
			shouldDelete := !danglingOnly
			if !shouldDelete {
				hasTag := false
				for _, ref := range refs {
					if _, ok := ref.(reference.NamedTagged); ok {
						hasTag = true
						break
					}
				}

				// Only delete if it's untagged (i.e. repo:<none>)
				shouldDelete = !hasTag
			}

			if shouldDelete {
				for _, ref := range refs {
					imgDel, err := daemon.ImageDelete(ref.String(), false, true)
					if imageDeleteFailed(ref.String(), err) {
						continue
					}
					deletedImages = append(deletedImages, imgDel...)
				}
			}
		} else {
			hex := id.Digest().Hex()
			imgDel, err := daemon.ImageDelete(hex, false, true)
			if imageDeleteFailed(hex, err) {
				continue
			}
			deletedImages = append(deletedImages, imgDel...)
		}

		rep.ImagesDeleted = append(rep.ImagesDeleted, deletedImages...)
	}

	// Compute how much space was freed
	for _, d := range rep.ImagesDeleted {
		if d.Deleted != "" {
			chid := layer.ChainID(d.Deleted)
			if l, ok := allLayers[chid]; ok {
				diffSize, err := l.DiffSize()
				if err != nil {
					logrus.Warnf("failed to get layer %s size: %v", chid, err)
					continue
				}
				rep.SpaceReclaimed += uint64(diffSize)
			}
		}
	}

	if canceled {
		logrus.Debugf("ImagesPrune operation cancelled: %#v", *rep)
	}

	return rep, nil
}

func imageDeleteFailed(ref string, err error) bool {
	switch {
	case err == nil:
		return false
	case errdefs.IsConflict(err):
		return true
	default:
		logrus.Warnf("failed to prune image %s: %v", ref, err)
		return true
	}
}
