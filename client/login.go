package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/registry"
)

type RegistryLoginOptions struct {
	Username      string
	Password      string
	ServerAddress string
	IdentityToken string
	RegistryToken string
}

// RegistryLoginResult holds the result of a RegistryLogin query.
type RegistryLoginResult struct {
	Auth registry.AuthResponse
}

// RegistryLogin authenticates the docker server with a given docker registry.
// It returns unauthorizedError when the authentication fails.
func (cli *Client) RegistryLogin(ctx context.Context, options RegistryLoginOptions) (RegistryLoginResult, error) {
	auth := registry.AuthConfig{
		Username:      options.Username,
		Password:      options.Password,
		ServerAddress: options.ServerAddress,
		IdentityToken: options.IdentityToken,
		RegistryToken: options.RegistryToken,
	}

	resp, err := cli.post(ctx, "/auth", url.Values{}, auth, nil)
	defer ensureReaderClosed(resp)

	if err != nil {
		return RegistryLoginResult{}, err
	}

	var response registry.AuthResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	return RegistryLoginResult{Auth: response}, err
}
