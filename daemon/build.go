package daemon

import (
	"context"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/events"
	c8dexporter "github.com/docker/docker/builder/builder-next/exporter"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageExportedByBuildkit is a callback that is called when an image is exported by buildkit.
// This is used to log the image creation event for untagged images.
// When no tag is given, buildkit doesn't call the image service so it has no
// way of knowing the image was created.
func (daemon *Daemon) ImageExportedByBuildkit(ctx context.Context, src *exporter.Source, attrs map[string]string) c8dexporter.ExportedCallback {
	if danglingPrefix, ok := attrs[string(exptypes.OptKeyDanglingPrefix)]; ok && danglingPrefix != "" {
		if imgSvc, ok := daemon.imageService.(interface {
			EnsureDanglingImage(ctx context.Context, refOrID string) error
		}); ok {
			imageName := attrs[string(exptypes.OptKeyName)]

			// The frontend can provide a name if requested.
			if imageName == "*" {
				imageName = string(src.Metadata["image.name"])
			}

			if imageName != "" {
				targetNames := strings.Split(imageName, ",")
				for _, targetName := range targetNames {
					if err := imgSvc.EnsureDanglingImage(ctx, targetName); err != nil && !cerrdefs.IsNotFound(err) {
						log.G(ctx).WithError(err).Warn("failed to keep the previous image as dangling")
					}
				}
			}
		}
	}
	return func(ctx context.Context, id string, desc ocispec.Descriptor) {
		daemon.imageService.LogImageEvent(ctx, id, id, events.ActionCreate)
	}
}

// ImageNamedByBuildkit is a callback that is called when an image is tagged by buildkit.
// Note: It is only called if the buildkit didn't call the image service itself to perform the tagging.
// Currently this only happens when the containerd image store is used.
func (daemon *Daemon) ImageNamedByBuildkit(ctx context.Context, ref reference.NamedTagged, desc ocispec.Descriptor) {
	daemon.imageService.LogImageEvent(ctx, desc.Digest.String(), reference.FamiliarString(ref), events.ActionTag)
}
