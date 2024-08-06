package containerd

import (
	"context"
	"time"

	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	imagetype "github.com/docker/docker/api/types/image"
	dimages "github.com/docker/docker/daemon/images"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ImageHistory returns a slice of HistoryResponseItem structures for the
// specified image name by walking the image lineage.
func (i *ImageService) ImageHistory(ctx context.Context, name string) ([]*imagetype.HistoryResponseItem, error) {
	start := time.Now()
	img, err := i.resolveImage(ctx, name)
	if err != nil {
		return nil, err
	}

	// TODO: pass platform in from the CLI
	pm := matchAllWithPreference(platforms.Default())

	im, err := i.getBestPresentImageManifest(ctx, img, pm)
	if err != nil {
		return nil, err
	}

	// Subset of ocispec.Image
	var ociImage struct {
		RootFS  ocispec.RootFS    `json:"rootfs"`
		History []ocispec.History `json:"history,omitempty"`
	}
	err = im.ReadConfig(ctx, &ociImage)
	if err != nil {
		return nil, err
	}

	var (
		history []*imagetype.HistoryResponseItem
		sizes   []int64
	)
	s := i.client.SnapshotService(i.snapshotter)

	diffIDs := ociImage.RootFS.DiffIDs
	for i := range diffIDs {
		chainID := identity.ChainID(diffIDs[0 : i+1]).String()

		use, err := s.Usage(ctx, chainID)
		if err != nil {
			return nil, err
		}

		sizes = append(sizes, use.Size)
	}

	for _, h := range ociImage.History {
		size := int64(0)
		if !h.EmptyLayer {
			if len(sizes) == 0 {
				return nil, errors.New("unable to find the size of the layer")
			}
			size = sizes[0]
			sizes = sizes[1:]
		}

		var created int64
		if h.Created != nil {
			created = h.Created.Unix()
		}
		history = append([]*imagetype.HistoryResponseItem{{
			ID:        "<missing>",
			Comment:   h.Comment,
			CreatedBy: h.CreatedBy,
			Created:   created,
			Size:      size,
			Tags:      nil,
		}}, history...)
	}

	findParents := func(img containerdimages.Image) []containerdimages.Image {
		imgs, err := i.getParentsByBuilderLabel(ctx, img)
		if err != nil {
			log.G(ctx).WithFields(log.Fields{
				"error": err,
				"image": img,
			}).Warn("failed to list parent images")
			return nil
		}
		return imgs
	}

	currentImg := img
	for _, h := range history {
		dgst := currentImg.Target.Digest.String()
		h.ID = dgst

		imgs, err := i.images.List(ctx, "target.digest=="+dgst)
		if err != nil {
			return nil, err
		}

		tags := getImageTags(ctx, imgs)
		h.Tags = append(h.Tags, tags...)

		parents := findParents(currentImg)

		foundNext := false
		for _, img := range parents {
			_, hasLabel := img.Labels[imageLabelClassicBuilderParent]
			if !foundNext || hasLabel {
				currentImg = img
				foundNext = true
			}
		}

		if !foundNext {
			break
		}
	}

	dimages.ImageActions.WithValues("history").UpdateSince(start)
	return history, nil
}

func getImageTags(ctx context.Context, imgs []containerdimages.Image) []string {
	var tags []string
	for _, img := range imgs {
		if isDanglingImage(img) {
			continue
		}

		name, err := reference.ParseNamed(img.Name)
		if err != nil {
			log.G(ctx).WithFields(log.Fields{
				"name":  name,
				"error": err,
			}).Warn("image with a name that's not a valid named reference")
			continue
		}

		tags = append(tags, reference.FamiliarString(name))
	}

	return tags
}

// getParentsByBuilderLabel finds images that were a base for the given image
// by an image label set by the legacy builder.
// NOTE: This only works for images built with legacy builder (not Buildkit).
func (i *ImageService) getParentsByBuilderLabel(ctx context.Context, img containerdimages.Image) ([]containerdimages.Image, error) {
	parent, ok := img.Labels[imageLabelClassicBuilderParent]
	if !ok || parent == "" {
		return nil, nil
	}

	dgst, err := digest.Parse(parent)
	if err != nil {
		log.G(ctx).WithFields(log.Fields{
			"error": err,
			"value": parent,
		}).Warnf("invalid %s label value", imageLabelClassicBuilderParent)
		return nil, nil
	}

	return i.images.List(ctx, "target.digest=="+dgst.String())
}
