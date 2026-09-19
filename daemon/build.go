package daemon

import (
	"context"
	"errors"
	"runtime"

	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/events"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (daemon *Daemon) validateBuildkitConfig(features map[string]bool) error {
	if !features["buildkit"] {
		return nil
	}

	// Check the actual image store, not the containerd-snapshotter feature flag:
	// storage-driver selection, migration, or a reload can make them differ.
	if runtime.GOOS == "windows" && !daemon.usesSnapshotter {
		return errors.New("features.buildkit=true requires the containerd image store on Windows")
	}

	log.G(context.TODO()).Warn("features.buildkit=true is unnecessary because BuildKit is already the default builder; remove this setting from the daemon configuration")
	return nil
}

// BuilderVersion returns the daemon's recommended builder version.
// BuildKit is preferred except on Windows with the classic image store.
// This is only a recommendation; clients choose which builder to use.
//
// Setting features.buildkit=false is an escape hatch to recommend the classic
// builder instead.
// CLI users can opt out per invocation with DOCKER_BUILDKIT=0.
// On Windows, using BuildKit requires the containerd image store.
func (daemon *Daemon) BuilderVersion() build.BuilderVersion {
	if enabled, ok := daemon.config().Features["buildkit"]; ok {
		if enabled {
			return build.BuilderBuildKit
		}
		return build.BuilderV1
	}
	if runtime.GOOS == "windows" && !daemon.usesSnapshotter {
		return build.BuilderV1
	}
	return build.BuilderBuildKit
}

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
