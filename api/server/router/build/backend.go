package build

import (
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"golang.org/x/net/context"
)

// Backend abstracts an image builder whose only purpose is to build an image referenced by an imageID.
type Backend interface {
	// Build builds a Docker image referenced by an imageID string.
	//
	// Note: Tagging an image should not be done by a Builder, it should instead be done
	// by the caller.
	//
	// TODO: make this return a reference instead of string
	BuildFromContext(ctx context.Context, src io.ReadCloser, remote string, buildOptions *types.ImageBuildOptions, pg backend.ProgressWriter) (string, error)
}
