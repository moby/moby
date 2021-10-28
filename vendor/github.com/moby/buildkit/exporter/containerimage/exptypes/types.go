package exptypes

import (
	srctypes "github.com/moby/buildkit/source/types"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	ExporterConfigDigestKey      = "config.digest"
	ExporterImageDigestKey       = "containerimage.digest"
	ExporterImageConfigKey       = "containerimage.config"
	ExporterImageConfigDigestKey = "containerimage.config.digest"
	ExporterInlineCache          = "containerimage.inlinecache"
	ExporterBuildInfo            = "containerimage.buildinfo"
	ExporterPlatformsKey         = "refs.platforms"
)

const EmptyGZLayer = digest.Digest("sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1")

type Platforms struct {
	Platforms []Platform
}

type Platform struct {
	ID       string
	Platform ocispecs.Platform
}

// BuildInfo defines build dependencies that will be added to image config as
// moby.buildkit.buildinfo.v1 key and returned in solver ExporterResponse as
// ExporterBuildInfo key.
type BuildInfo struct {
	// Type defines the BuildInfoType source type (docker-image, git, http).
	Type BuildInfoType `json:"type,omitempty"`
	// Ref is the reference of the source.
	Ref string `json:"ref,omitempty"`
	// Alias is a special field used to match with the actual source ref
	// because frontend might have already transformed a string user typed
	// before generating LLB.
	Alias string `json:"alias,omitempty"`
	// Pin is the source digest.
	Pin string `json:"pin,omitempty"`
}

// BuildInfoType contains source type.
type BuildInfoType string

// List of source types.
const (
	BuildInfoTypeDockerImage BuildInfoType = srctypes.DockerImageScheme
	BuildInfoTypeGit         BuildInfoType = srctypes.GitScheme
	BuildInfoTypeHTTP        BuildInfoType = srctypes.HTTPScheme
)
