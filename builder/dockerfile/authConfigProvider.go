package dockerfile

import (
	"context"

	"strings"
	"time"

	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client/session"
	"github.com/docker/docker/client/session/authsession"
	"github.com/docker/docker/registry"
)

type AuthConfigProvider interface {
	GetAuthenticatorForRegistry(registry string) auth.Authenticator
}
type authConfigProvider struct {
	config map[string]types.AuthConfig
	caller session.Caller
}

func NewAuthConfigProvider(originalConfig map[string]types.AuthConfig, caller session.Caller) AuthConfigProvider {
	if originalConfig == nil {
		originalConfig = make(map[string]types.AuthConfig)
	}
	if caller != nil && !caller.Supports("_main", "GetAuth") {
		caller = nil
	}
	return &authConfigProvider{
		config: originalConfig,
		caller: caller,
	}
}

func (a *authConfigProvider) getAuthConfig(includingRegistries ...string) (map[string]types.AuthConfig, error) {
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
func (a *authConfigProvider) GetAuthenticatorForRegistry(r string) auth.Authenticator {
	if a.caller == nil {
		conf := a.config[r]
		return registry.NewAuthConfigAuthenticator(&conf)
	}
	return &remoteAuthenticator{a, r, make(map[string]*authsession.GetTokenResponse)}
}

type remoteAuthenticator struct {
	provider   *authConfigProvider
	registry   string
	tokenCache map[string]*authsession.GetTokenResponse
}

func (a *remoteAuthenticator) getRemoteAuthConfig() types.AuthConfig {
	cs, err := a.provider.getAuthConfig(a.registry)
	if err != nil {
		return types.AuthConfig{}
	}
	return cs[a.registry]
}
func (a *remoteAuthenticator) GetBasicAuthInfo() (username, password string) {
	c := a.getRemoteAuthConfig()
	return c.Username, c.Password
}
func (a *remoteAuthenticator) HasBasicAuthInfo() bool {
	u, p := a.GetBasicAuthInfo()
	return u != "" && p != ""
}
func (a *remoteAuthenticator) HasUsername() bool {
	c := a.getRemoteAuthConfig()
	return c.Username != ""
}
func (a *remoteAuthenticator) GetUsername() string {
	c := a.getRemoteAuthConfig()
	return c.Username
}
func (a *remoteAuthenticator) HasIdentityTokenInfo() bool {
	u, i := a.GetIdentityTokenInfo()
	return u != "" && i != ""
}
func (a *remoteAuthenticator) GetIdentityTokenInfo() (username, identityToken string) {
	c := a.getRemoteAuthConfig()
	return c.Username, c.IdentityToken
}
func (a *remoteAuthenticator) HasPassthroughToken() bool {
	return false
}
func (a *remoteAuthenticator) GetPassthroughToken() string {
	return ""
}
func (a *remoteAuthenticator) GetAccessToken(realm, service, clientID string, scopes []string, skipCache, offlineAccess, forceOAuth bool) (token string, err error) {

	cacheKeyParts := []string{realm, service, clientID}
	cacheKeyParts = append(cacheKeyParts, scopes...)
	cacheKey := strings.Join(cacheKeyParts, "|")
	if !skipCache {
		if cache, ok := a.tokenCache[cacheKey]; ok {
			if time.Unix(cache.IssuedAt, 0).Add(time.Duration(cache.ExpiresIn) * time.Second).After(time.Now()) {
				return cache.AccessToken, nil
			}
		}
	}
	s, err := a.provider.caller.Call(context.Background(), "_main", "GetAccessToken", make(map[string][]string))
	if err != nil {
		return "", err
	}
	tokenRequest := authsession.GetTokenRequest{
		ClientID:      clientID,
		ForceOAuth:    forceOAuth,
		OfflineAccess: offlineAccess,
		Realm:         realm,
		Registry:      a.registry,
		Scopes:        scopes,
		Service:       service,
		SkipCache:     skipCache,
	}

	err = s.SendMsg(&tokenRequest)
	if err != nil {
		return "", err
	}
	tokenResponse := new(authsession.GetTokenResponse)

	err = s.RecvMsg(tokenResponse)

	if err != nil {
		return "", err
	}

	if !skipCache {
		a.tokenCache[cacheKey] = tokenResponse
	}
	return tokenResponse.AccessToken, nil
}
