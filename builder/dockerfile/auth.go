package dockerfile

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client/session/auth"
)

type AuthConfigProvider interface {
	GetAuthConfig(registry string) (*types.AuthConfig, error)
}

type staticAuthConfigProvider struct {
	auths map[string]types.AuthConfig
}

func (p *staticAuthConfigProvider) GetAuthConfig(registry string) (*types.AuthConfig, error) {
	if a, ok := p.auths[registry]; ok {
		return &a, nil
	}
	return nil, nil
}

type streamingAuthConfigProvider struct {
	client auth.AuthConfigProviderClient
}

func (p *streamingAuthConfigProvider) GetAuthConfig(registry string) (*types.AuthConfig, error) {
	res, err := p.client.GetRegistryAuth(context.Background(), &auth.RegistryAuthRequest{Registry: registry})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return &types.AuthConfig{
		Auth:          res.Auth,
		Email:         res.Email,
		IdentityToken: res.IdentityToken,
		Password:      res.Password,
		RegistryToken: res.RegistryToken,
		ServerAddress: res.ServerAddress,
		Username:      res.Username,
	}, nil
}
