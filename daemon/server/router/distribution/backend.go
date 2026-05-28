package distribution

import (
	"context"

	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/moby/moby/api/types/registry"
)

// Backend is all the methods that need to be implemented
// to provide image specific functionality.
type Backend interface {
	GetRepositories(context.Context, reference.Named, *registry.AuthConfig) ([]distribution.Repository, error)
}
