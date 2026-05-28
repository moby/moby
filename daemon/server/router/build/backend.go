package build

import (
	"context"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/v2/daemon/server/buildbackend"
)

// Backend abstracts an image builder whose only purpose is to build an image referenced by an imageID.
type Backend interface {
	// Build a Docker image returning the id of the image
	// TODO: make this return a reference instead of string
	Build(context.Context, buildbackend.BuildConfig) (string, error)

	// PruneCache prunes the build cache.
	PruneCache(context.Context, buildbackend.CachePruneOptions) (*build.CachePruneReport, error)
	Cancel(context.Context, string) error
}

type experimentalProvider interface {
	HasExperimental() bool
}
