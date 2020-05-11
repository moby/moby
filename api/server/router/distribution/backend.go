package distribution // import "github.com/docker/docker/api/server/router/distribution"

import (
	"context"

	reference "github.com/containerd/containerd/reference/docker"
	"github.com/docker/distribution"
	"github.com/docker/docker/api/types/registry"
)

// Backend is all the methods that need to be implemented
// to provide image specific functionality.
type Backend interface {
	GetRepository(context.Context, reference.Named, *registry.AuthConfig) (distribution.Repository, error)
}
