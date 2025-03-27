package distribution // import "github.com/docker/docker/api/server/router/distribution"

import (
	"context"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/internal/registryclient"
)

// Backend is all the methods that need to be implemented
// to provide image specific functionality.
type Backend interface {
	GetRepositories(context.Context, reference.Named, *registry.AuthConfig) ([]registryclient.Repository, error)
}
