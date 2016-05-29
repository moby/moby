package daemon

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/dockerversion"
	"github.com/docker/engine-api/types"
)

// AuthenticateToRegistry checks the validity of credentials in authConfig
func (daemon *Daemon) AuthenticateToRegistry(ctx context.Context, authConfig *types.AuthConfig) (string, string, error) {
	return daemon.RegistryService.Auth(ctx, authConfig, dockerversion.DockerUserAgent(ctx))
}
