package distribution

import (
	"context"

	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/v2/daemon/internal/registryclient"
)

// Backend is all the methods that need to be implemented
// to provide image specific functionality.
type Backend interface {
	GetRepositories(context.Context, reference.Named, *registry.AuthConfig) ([]registryclient.Repository, error)
}
