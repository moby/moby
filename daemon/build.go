package daemon

import (
	"context"

	"github.com/docker/docker/api/types/events"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageExportedByBuildkit is a callback that is called when an image is exported by buildkit.
// This is used to log the image creation event for untagged images.
// When no tag is given, buildkit doesn't call the image service so it has no
// way of knowing the image was created.
func (daemon *Daemon) ImageExportedByBuildkit(ctx context.Context, id string, desc ocispec.Descriptor) {
	daemon.imageService.LogImageEvent(ctx, id, id, events.ActionCreate)
}
