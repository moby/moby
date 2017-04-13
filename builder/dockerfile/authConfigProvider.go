package dockerfile

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client/session"
	"github.com/docker/docker/client/session/authsession"
)

type AuthConfigProvider interface {
	GetAuthConfig(includingRegistries ...string) (map[string]types.AuthConfig, error)
}
type authConfigProvider struct {
	config map[string]types.AuthConfig
	caller session.Caller
}

func NewAuthConfigProvider(originalConfig map[string]types.AuthConfig, caller session.Caller) AuthConfigProvider {
	if originalConfig == nil {
		originalConfig = make(map[string]types.AuthConfig)
	}
	return &authConfigProvider{
		config: originalConfig,
		caller: caller,
	}
}

func (a *authConfigProvider) GetAuthConfig(includingRegistries ...string) (map[string]types.AuthConfig, error) {
	var missingRegistries []string
	for _, registry := range includingRegistries {
		if _, ok := a.config[registry]; !ok {
			missingRegistries = append(missingRegistries, registry)
		}
	}
	if len(missingRegistries) > 0 && a.caller != nil {
		opts := make(map[string][]string)
		opts["registries"] = missingRegistries
		s, err := a.caller.Call(context.Background(), "_main", "GetAuth", opts)
		if err != nil {
			return nil, err
		}
		results := new(authsession.AuthConfigs)
		if err = s.RecvMsg(results); err != nil {
			return nil, err
		}
		for key, val := range results.Auths {
			a.config[key] = types.AuthConfig{
				Auth:          val.Auth,
				Email:         val.Email,
				IdentityToken: val.IdentityToken,
				Password:      val.Password,
				RegistryToken: val.RegistryToken,
				ServerAddress: val.ServerAddress,
				Username:      val.Username,
			}
		}
	}
	return a.config, nil
}
