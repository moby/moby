package containerd

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/snapshots"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/opencontainers/image-spec/identity"
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

	snapshotter := i.client.SnapshotService(containerd.DefaultSnapshotter)

	var summaries []*types.ImageSummary
	for _, img := range imgs {
		if !filter(img) {
			continue
		}

		size, err := img.Size(ctx)
		if err != nil {
			return nil, err
		}

		virtualSize, err := computeVirtualSize(ctx, img, snapshotter)
		if err != nil {
			return nil, err
		}

		summaries = append(summaries, &types.ImageSummary{
			ParentID:    "",
			ID:          img.Target().Digest.String(),
			Created:     img.Metadata().CreatedAt.Unix(),
			RepoDigests: []string{img.Name() + "@" + img.Target().Digest.String()}, // "hello-world@sha256:bfea6278a0a267fad2634554f4f0c6f31981eea41c553fdf5a83e95a41d40c38"},
			RepoTags:    []string{img.Name()},
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

func computeVirtualSize(ctx context.Context, image containerd.Image, snapshotter snapshots.Snapshotter) (int64, error) {
	var virtualSize int64
	diffIDs, err := image.RootFS(ctx)
	if err != nil {
		return virtualSize, err
	}
	for _, chainID := range identity.ChainIDs(diffIDs) {
		usage, err := snapshotter.Usage(ctx, chainID.String())
		if err != nil {
			return virtualSize, err
		}
		virtualSize += usage.Size
	}
	return virtualSize, nil
}
