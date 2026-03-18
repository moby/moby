package daemon

import (
	"context"

	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/v2/dockerversion"
)

// AuthenticateToRegistry checks the validity of credentials in authConfig
func (daemon *Daemon) AuthenticateToRegistry(ctx context.Context, authConfig *registry.AuthConfig) (string, error) {
	return daemon.registryService.Auth(ctx, authConfig, dockerversion.DockerUserAgent(ctx))
}
