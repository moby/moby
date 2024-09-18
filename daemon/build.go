package daemon

import (
	"context"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/events"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageExportedByBuildkit is a callback that is called when an image is exported by buildkit.
// This is used to log the image creation event for untagged images.
// When no tag is given, buildkit doesn't call the image service so it has no
// way of knowing the image was created.
func (daemon *Daemon) ImageExportedByBuildkit(ctx context.Context, id string, desc ocispec.Descriptor) {
	daemon.imageService.LogImageEvent(id, id, events.ActionCreate)
}

// ImageNamedByBuildkit is a callback that is called when an image is tagged by buildkit.
// Note: It is only called if the buildkit didn't call the image service itself to perform the tagging.
// Currently this only happens when the containerd image store is used.
func (daemon *Daemon) ImageNamedByBuildkit(ctx context.Context, ref reference.NamedTagged, desc ocispec.Descriptor) {
	id := desc.Digest.String()
	name := reference.FamiliarString(ref)
	daemon.imageService.LogImageEvent(id, name, events.ActionTag)
}
