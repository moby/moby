package containerd

import (
	"context"
	"sort"

	"github.com/containerd/containerd/images"
	containerdimages "github.com/containerd/containerd/images"
	cplatforms "github.com/containerd/containerd/platforms"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	imagetype "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/platforms"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ImageHistory returns a slice of HistoryResponseItem structures for the
// specified image name by walking the image lineage.
func (i *ImageService) ImageHistory(ctx context.Context, name string) ([]*imagetype.HistoryResponseItem, error) {
	img, err := i.resolveImage(ctx, name)
	if err != nil {
		return nil, err
	}

	cs := i.client.ContentStore()
	// TODO: pass platform in from the CLI
	platform := platforms.AllPlatformsWithPreference(cplatforms.Default())

	var presentImages []ocispec.Image
	err = i.walkImageManifests(ctx, img, func(img *ImageManifest) error {
		conf, err := img.Config(ctx)
		if err != nil {
			return err
		}
		var ociimage ocispec.Image
		if err := readConfig(ctx, cs, conf, &ociimage); err != nil {
			return err
		}
		presentImages = append(presentImages, ociimage)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(presentImages) == 0 {
		return nil, errdefs.NotFound(errors.New("failed to find image manifest"))
	}

	sort.SliceStable(presentImages, func(i, j int) bool {
		return platform.Less(presentImages[i].Platform, presentImages[j].Platform)
	})
	ociimage := presentImages[0]

	var (
		history []*imagetype.HistoryResponseItem
		sizes   []int64
	)
	s := i.client.SnapshotService(i.snapshotter)

	diffIDs := ociimage.RootFS.DiffIDs
	for i := range diffIDs {
		chainID := identity.ChainID(diffIDs[0 : i+1]).String()

		use, err := s.Usage(ctx, chainID)
		if err != nil {
			return nil, err
		}

		sizes = append(sizes, use.Size)
	}

	for _, h := range ociimage.History {
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

	findParents := func(img images.Image) []images.Image {
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

	is := i.client.ImageService()
	currentImg := img
	for _, h := range history {
		dgst := currentImg.Target.Digest.String()
		h.ID = dgst

		imgs, err := is.List(ctx, "target.digest=="+dgst)
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

	return history, nil
}

func getImageTags(ctx context.Context, imgs []images.Image) []string {
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

	return i.client.ImageService().List(ctx, "target.digest=="+dgst.String())
}
