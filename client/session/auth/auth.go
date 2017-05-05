package auth

import (
	"context"

	xcontext "golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client/session"
	"github.com/docker/docker/client/session/auth/internal"
)

type AuthConfigProvider interface {
	GetRegistryAuth(ctx context.Context, registry string) (*types.AuthConfig, error)
}

type authConfigProviderServerWrapper struct {
	impl AuthConfigProvider
}

func (w *authConfigProviderServerWrapper) GetRegistryAuth(ctx xcontext.Context, req *internal.RegistryAuthRequest) (*internal.AuthConfig, error) {
	var reg string
	if req != nil {
		reg = req.Registry
	}
	var conf *types.AuthConfig
	conf, err := w.impl.GetRegistryAuth(ctx, reg)
	if err != nil {
		return nil, err
	}
	if conf != nil {
		return &internal.AuthConfig{
			Auth:          conf.Auth,
			Email:         conf.Email,
			IdentityToken: conf.IdentityToken,
			Password:      conf.Password,
			RegistryToken: conf.RegistryToken,
			ServerAddress: conf.ServerAddress,
			Username:      conf.Username,
		}, nil
	}
	return nil, nil
}

func AttachAuthConfigProviderToSession(p AuthConfigProvider, sess *session.ServerSession) {
	sess.Allow(internal.AuthConfigProviderServiceDesc(), &authConfigProviderServerWrapper{p})
}

type authConfigProviderClientWrapper struct {
	impl internal.AuthConfigProviderClient
}

func (w *authConfigProviderClientWrapper) GetRegistryAuth(ctx context.Context, registry string) (result *types.AuthConfig, err error) {
	var rawResult *internal.AuthConfig
	rawResult, err = w.impl.GetRegistryAuth(ctx, &internal.RegistryAuthRequest{Registry: registry})
	if rawResult != nil {
		result = &types.AuthConfig{
			Auth:          rawResult.Auth,
			Email:         rawResult.Email,
			IdentityToken: rawResult.IdentityToken,
			Password:      rawResult.Password,
			RegistryToken: rawResult.RegistryToken,
			ServerAddress: rawResult.ServerAddress,
			Username:      rawResult.Username,
		}
	}
	return
}

func TryGetAuthConfigProviderClient(c session.Caller) (AuthConfigProvider, bool) {
	if !c.Supports(internal.AuthConfigProviderServiceDesc().ServiceName) {
		return nil, false
	}
	rawCli := internal.NewAuthConfigProviderClient(c.GetGrpcConn())
	return &authConfigProviderClientWrapper{impl: rawCli}, true
}
