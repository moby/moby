package containerd

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	"github.com/sirupsen/logrus"
)

var acceptedImageFilterTags = map[string]bool{
	"dangling":  false, // TODO(thaJeztah): implement "dangling" filter: see https://github.com/moby/moby/issues/43846
	"label":     true,
	"before":    true,
	"since":     true,
	"reference": false, // TODO(thaJeztah): implement "reference" filter: see https://github.com/moby/moby/issues/43847
}

// Images returns a filtered list of images.
//
// TODO(thaJeztah): sort the results by created (descending); see https://github.com/moby/moby/issues/43848
// TODO(thaJeztah): implement opts.ContainerCount (used for docker system df); see https://github.com/moby/moby/issues/43853
// TODO(thaJeztah): add labels to results; see https://github.com/moby/moby/issues/43852
// TODO(thaJeztah): verify behavior of `RepoDigests` and `RepoTags` for images without (untagged) or multiple tags; see https://github.com/moby/moby/issues/43861
// TODO(thaJeztah): verify "Size" vs "VirtualSize" in images; see https://github.com/moby/moby/issues/43862
func (i *ImageService) Images(ctx context.Context, opts types.ImageListOptions) ([]*types.ImageSummary, error) {
	if err := opts.Filters.Validate(acceptedImageFilterTags); err != nil {
		return nil, err
	}

	filter, err := i.setupFilters(ctx, opts.Filters)
	if err != nil {
		return nil, err
	}

	imgs, err := i.client.ListImages(ctx)
	if err != nil {
		return nil, err
	}

	snapshotter := i.client.SnapshotService(i.snapshotter)
	sizeCache := make(map[digest.Digest]int64)
	snapshotSizeFn := func(d digest.Digest) (int64, error) {
		if s, ok := sizeCache[d]; ok {
			return s, nil
		}
		usage, err := snapshotter.Usage(ctx, d.String())
		if err != nil {
			return 0, err
		}
		sizeCache[d] = usage.Size
		return usage.Size, nil
	}

	var (
		summaries = make([]*types.ImageSummary, 0, len(imgs))
		root      []*[]digest.Digest
		layers    map[digest.Digest]int
	)
	if opts.SharedSize {
		root = make([]*[]digest.Digest, len(imgs))
		layers = make(map[digest.Digest]int)
	}

	for n, img := range imgs {
		if !filter(img) {
			continue
		}

		diffIDs, err := img.RootFS(ctx)
		if err != nil {
			return nil, err
		}
		chainIDs := identity.ChainIDs(diffIDs)
		if opts.SharedSize {
			root[n] = &chainIDs
			for _, id := range chainIDs {
				layers[id] = layers[id] + 1
			}
		}

		size, err := img.Size(ctx)
		if err != nil {
			return nil, err
		}

		virtualSize, err := computeVirtualSize(chainIDs, snapshotSizeFn)
		if err != nil {
			return nil, err
		}

		var repoTags, repoDigests []string
		rawImg := img.Metadata()
		target := rawImg.Target.Digest

		logger := logrus.WithFields(logrus.Fields{
			"name":   img.Name(),
			"digest": target,
		})

		ref, err := reference.ParseNamed(rawImg.Name)
		if err != nil {
			// If the image has unexpected name format (not a Named reference or a dangling image)
			// add the offending name to RepoTags but also log an error to make it clear to the
			// administrator that this is unexpected.
			// TODO: Reconsider when containerd is more strict on image names, see:
			//       https://github.com/containerd/containerd/issues/7986
			if !isDanglingImage(rawImg) {
				logger.WithError(err).Error("failed to parse image name as reference")
				repoTags = append(repoTags, img.Name())
			}
		} else {
			repoTags = append(repoTags, reference.TagNameOnly(ref).String())

			digested, err := reference.WithDigest(reference.TrimNamed(ref), target)
			if err != nil {
				logger.WithError(err).Error("failed to create digested reference")
			} else {
				repoDigests = append(repoDigests, digested.String())
			}
		}

		summaries = append(summaries, &types.ImageSummary{
			ParentID:    "",
			ID:          target.String(),
			Created:     rawImg.CreatedAt.Unix(),
			RepoDigests: repoDigests,
			RepoTags:    repoTags,
			Size:        size,
			VirtualSize: virtualSize,
			// -1 indicates that the value has not been set (avoids ambiguity
			// between 0 (default) and "not set". We cannot use a pointer (nil)
			// for this, as the JSON representation uses "omitempty", which would
			// consider both "0" and "nil" to be "empty".
			SharedSize: -1,
			Containers: -1,
		})
	}

	if opts.SharedSize {
		for n, chainIDs := range root {
			sharedSize, err := computeSharedSize(*chainIDs, layers, snapshotSizeFn)
			if err != nil {
				return nil, err
			}
			summaries[n].SharedSize = sharedSize
		}
	}

	return summaries, nil
}

type imageFilterFunc func(image containerd.Image) bool

// setupFilters constructs an imageFilterFunc from the given imageFilters.
//
// TODO(thaJeztah): reimplement filters using containerd filters: see https://github.com/moby/moby/issues/43845
func (i *ImageService) setupFilters(ctx context.Context, imageFilters filters.Args) (imageFilterFunc, error) {
	var fltrs []imageFilterFunc
	err := imageFilters.WalkValues("before", func(value string) error {
		ref, err := reference.ParseDockerRef(value)
		if err != nil {
			return err
		}
		img, err := i.client.GetImage(ctx, ref.String())
		if img != nil {
			t := img.Metadata().CreatedAt
			fltrs = append(fltrs, func(image containerd.Image) bool {
				created := image.Metadata().CreatedAt
				return created.Equal(t) || created.After(t)
			})
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	err = imageFilters.WalkValues("since", func(value string) error {
		ref, err := reference.ParseDockerRef(value)
		if err != nil {
			return err
		}
		img, err := i.client.GetImage(ctx, ref.String())
		if img != nil {
			t := img.Metadata().CreatedAt
			fltrs = append(fltrs, func(image containerd.Image) bool {
				created := image.Metadata().CreatedAt
				return created.Equal(t) || created.Before(t)
			})
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	if imageFilters.Contains("label") {
		fltrs = append(fltrs, func(image containerd.Image) bool {
			return imageFilters.MatchKVList("label", image.Labels())
		})
	}
	return func(image containerd.Image) bool {
		for _, filter := range fltrs {
			if !filter(image) {
				return false
			}
		}
		return true
	}, nil
}

func computeVirtualSize(chainIDs []digest.Digest, sizeFn func(d digest.Digest) (int64, error)) (int64, error) {
	var virtualSize int64
	for _, chainID := range chainIDs {
		size, err := sizeFn(chainID)
		if err != nil {
			return virtualSize, err
		}
		virtualSize += size
	}
	return virtualSize, nil
}

func computeSharedSize(chainIDs []digest.Digest, layers map[digest.Digest]int, sizeFn func(d digest.Digest) (int64, error)) (int64, error) {
	var sharedSize int64
	for _, chainID := range chainIDs {
		if layers[chainID] == 1 {
			continue
		}
		size, err := sizeFn(chainID)
		if err != nil {
			return 0, err
		}
		sharedSize += size
	}
	return sharedSize, nil
}
