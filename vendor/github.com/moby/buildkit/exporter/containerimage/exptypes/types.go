package exptypes

import (
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	ExporterConfigDigestKey      = "config.digest"
	ExporterImageDigestKey       = "containerimage.digest"
	ExporterImageConfigKey       = "containerimage.config"
	ExporterImageConfigDigestKey = "containerimage.config.digest"
	ExporterImageDescriptorKey   = "containerimage.descriptor"
	ExporterInlineCache          = "containerimage.inlinecache"
	ExporterBuildInfo            = "containerimage.buildinfo" // Deprecated: Build information is deprecated: https://github.com/moby/buildkit/blob/master/docs/deprecated.md
	ExporterPlatformsKey         = "refs.platforms"
	ExporterEpochKey             = "source.date.epoch"
)

// KnownRefMetadataKeys are the subset of exporter keys that can be suffixed by
// a platform to become platform specific
var KnownRefMetadataKeys = []string{
	ExporterImageConfigKey,
	ExporterInlineCache,
	ExporterBuildInfo,
}

type Platforms struct {
	Platforms []Platform
}

type Platform struct {
	ID       string
	Platform ocispecs.Platform
}
