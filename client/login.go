package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/docker/docker/api/types/auth"
	"github.com/docker/docker/api/types/registry"
)

// RegistryLogin authenticates the docker server with a given docker registry.
// It returns unauthorizedError when the authentication fails.
func (cli *Client) RegistryLogin(ctx context.Context, auth auth.Config) (registry.AuthenticateOKBody, error) {
	resp, err := cli.post(ctx, "/auth", url.Values{}, auth, nil)

	if resp.statusCode == http.StatusUnauthorized {
		return registry.AuthenticateOKBody{}, unauthorizedError{err}
	}
	if err != nil {
		return registry.AuthenticateOKBody{}, err
	}

	var response registry.AuthenticateOKBody
	err = json.NewDecoder(resp.body).Decode(&response)
	ensureReaderClosed(resp)
	return response, err
}
