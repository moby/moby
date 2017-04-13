package authsession

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client/session"
	"github.com/pkg/errors"
)

type AuthConfigProvider interface {
	GetAuthConfig(registry string) types.AuthConfig
}
type authConfigHandler struct {
	name            string
	provider        AuthConfigProvider
	registryAuthLog func(string)
}

func NewAuthconfigHandler(name string, provider AuthConfigProvider, registryAuthLog func(string)) session.Attachment {
	h := &authConfigHandler{
		name:            name,
		provider:        provider,
		registryAuthLog: registryAuthLog,
	}
	return h
}

func (h *authConfigHandler) RegisterHandlers(fn func(id, method string) error) error {
	return fn(h.name, "GetAuth")
}
func (h *authConfigHandler) Handle(ctx context.Context, id, method string, opts map[string][]string, stream session.Stream) error {
	if id != h.name {
		return errors.Errorf("invalid id %s", id)
	}
	if method != "GetAuth" {
		return errors.Errorf("unknown method %s", method)
	}
	registries := opts["registries"]
	auths := new(AuthConfigs)
	auths.Auths = make(map[string]*AuthConfig)
	for _, registry := range registries {
		if h.registryAuthLog != nil {
			h.registryAuthLog(registry)
		}
		c := h.provider.GetAuthConfig(registry)
		auths.Auths[registry] = &AuthConfig{
			Auth:          c.Auth,
			Email:         c.Email,
			IdentityToken: c.IdentityToken,
			Password:      c.Password,
			RegistryToken: c.RegistryToken,
			ServerAddress: c.ServerAddress,
			Username:      c.Username,
		}
	}
	return stream.SendMsg(auths)
}
