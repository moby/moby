package containerd

import (
	"context"
	"sort"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	cplatforms "github.com/containerd/containerd/platforms"
	"github.com/distribution/reference"
	imagetype "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/platforms"
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

		history = append([]*imagetype.HistoryResponseItem{{
			ID:        "<missing>",
			Comment:   h.Comment,
			CreatedBy: h.CreatedBy,
			Created:   h.Created.Unix(),
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
			}).Warnf("failed to list parent images")
			return nil
		}
		return imgs
	}

	currentImg := img
	for _, h := range history {
		h.ID = currentImg.Target.Digest.String()
		imgs := findParents(currentImg)

		foundNext := false
		for _, img := range imgs {
			if _, ok := img.Labels[imageLabelClassicBuilderParent]; ok {
				currentImg = img
				foundNext = true
			}

			if isDanglingImage(img) {
				continue
			}
			name, err := reference.ParseNamed(img.Name)
			if err != nil {
				return nil, err
			}
			h.Tags = append(h.Tags, reference.FamiliarString(name))
		}

		if !foundNext {
			break
		}
	}

	return history, nil
}
