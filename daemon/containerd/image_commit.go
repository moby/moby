package containerd

import (
	"context"
	"errors"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
)

// CommitImage creates a new image from a commit config.
func (i *ImageService) CommitImage(ctx context.Context, c backend.CommitConfig) (image.ID, error) {
	return "", errdefs.NotImplemented(errors.New("not implemented"))
}

// CommitBuildStep is used by the builder to create an image for each step in
// the build.
//
// This method is different from CreateImageFromContainer:
//   - it doesn't attempt to validate container state
//   - it doesn't send a commit action to metrics
//   - it doesn't log a container commit event
//
// This is a temporary shim. Should be removed when builder stops using commit.
func (i *ImageService) CommitBuildStep(ctx context.Context, c backend.CommitConfig) (image.ID, error) {
	return "", errdefs.NotImplemented(errors.New("not implemented"))
}
