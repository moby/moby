package build

import "github.com/moby/moby/api/types/build"

// BuilderVersion sets the version of underlying builder to use
type BuilderVersion = build.BuilderVersion

const (
	// BuilderV1 is the first generation builder in docker daemon
	BuilderV1 = build.BuilderV1
	// BuilderBuildKit is builder based on moby/buildkit project
	BuilderBuildKit = build.BuilderBuildKit
)

// Result contains the image id of a successful build.
type Result = build.Result

// ImageBuildOptions holds the information
// necessary to build images.
type ImageBuildOptions = build.ImageBuildOptions

// ImageBuildOutput defines configuration for exporting a build result
type ImageBuildOutput = build.ImageBuildOutput

// ImageBuildResponse holds information
// returned by a server after building
// an image.
type ImageBuildResponse = build.ImageBuildResponse

// CacheRecord contains information about a build cache record.
type CacheRecord = build.CacheRecord

// CachePruneOptions hold parameters to prune the build cache.
type CachePruneOptions = build.CachePruneOptions

// CachePruneReport contains the response for Engine API:
// POST "/build/prune"
type CachePruneReport = build.CachePruneReport

// CacheDiskUsage contains disk usage for the build cache.
type CacheDiskUsage = build.CacheDiskUsage
