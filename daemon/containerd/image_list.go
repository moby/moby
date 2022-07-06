package containerd

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/snapshots"
	"github.com/docker/docker/api/types"
	"github.com/opencontainers/image-spec/identity"
)

// Images returns a filtered list of images.
func (i *ImageService) Images(ctx context.Context, opts types.ImageListOptions) ([]*types.ImageSummary, error) {
	imgs, err := i.client.ListImages(ctx)
	if err != nil {
		return nil, err
	}

	snapshotter := i.client.SnapshotService(containerd.DefaultSnapshotter)

	var ret []*types.ImageSummary
	for _, img := range imgs {
		size, err := img.Size(ctx)
		if err != nil {
			return nil, err
		}

		virtualSize, err := computeVirtualSize(ctx, img, snapshotter)
		if err != nil {
			return nil, err
		}

		ret = append(ret, &types.ImageSummary{
			RepoDigests: []string{img.Name() + "@" + img.Target().Digest.String()}, // "hello-world@sha256:bfea6278a0a267fad2634554f4f0c6f31981eea41c553fdf5a83e95a41d40c38"},
			RepoTags:    []string{img.Name()},
			Containers:  -1,
			ParentID:    "",
			SharedSize:  -1,
			VirtualSize: virtualSize,
			ID:          img.Target().Digest.String(),
			Created:     img.Metadata().CreatedAt.Unix(),
			Size:        size,
		})
	}

	return ret, nil
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
