package distribution // import "github.com/moby/moby/api/server/router/distribution"

import (
	"context"

	"github.com/docker/distribution"
	"github.com/docker/distribution/reference"
	"github.com/moby/moby/api/types"
)

// Backend is all the methods that need to be implemented
// to provide image specific functionality.
type Backend interface {
	GetRepository(context.Context, reference.Named, *types.AuthConfig) (distribution.Repository, error)
}
