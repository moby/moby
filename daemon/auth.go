package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/dockerversion"
)

// AuthenticateToRegistry checks the validity of credentials in authConfig
func (daemon *Daemon) AuthenticateToRegistry(ctx context.Context, authConfig *registry.AuthConfig) (string, string, error) {
	return daemon.registryService.Auth(ctx, authConfig, dockerversion.DockerUserAgent(ctx))
}
