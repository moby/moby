package daemon

import (
	"context"

	"github.com/distribution/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/moby/moby/api/types/events"
)

// ImageExportedByBuildkit is a callback that is called when an image is exported by buildkit.
// This is used to log the image creation event for untagged images.
// When no tag is given, buildkit doesn't call the image service so it has no
// way of knowing the image was created.
func (daemon *Daemon) ImageExportedByBuildkit(ctx context.Context, id string, desc ocispec.Descriptor) {
	daemon.imageService.LogImageEvent(ctx, id, id, events.ActionCreate)
}

// ImageNamedByBuildkit is a callback that is called when an image is tagged by buildkit.
// Note: It is only called if the buildkit didn't call the image service itself to perform the tagging.
// Currently this only happens when the containerd image store is used.
func (daemon *Daemon) ImageNamedByBuildkit(ctx context.Context, ref reference.NamedTagged, desc ocispec.Descriptor) {
	daemon.imageService.LogImageEvent(ctx, desc.Digest.String(), reference.FamiliarString(ref), events.ActionTag)
}
