package containerd

import (
	"context"

	"github.com/docker/docker/api/types"
)

// Images returns a filtered list of images.
func (i *ImageService) Images(ctx context.Context, opts types.ImageListOptions) ([]*types.ImageSummary, error) {
	imgs, err := i.client.ListImages(ctx)
	if err != nil {
		return nil, err
	}

	var ret []*types.ImageSummary
	for _, img := range imgs {
		size, err := img.Size(ctx)
		if err != nil {
			return nil, err
		}

		ret = append(ret, &types.ImageSummary{
			RepoDigests: []string{img.Name() + "@" + img.Target().Digest.String()}, // "hello-world@sha256:bfea6278a0a267fad2634554f4f0c6f31981eea41c553fdf5a83e95a41d40c38"},
			RepoTags:    []string{img.Name()},
			Containers:  -1,
			ParentID:    "",
			SharedSize:  -1,
			VirtualSize: 10,
			ID:          img.Target().Digest.String(),
			Created:     img.Metadata().CreatedAt.Unix(),
			Size:        size,
		})
	}

	return ret, nil
}
