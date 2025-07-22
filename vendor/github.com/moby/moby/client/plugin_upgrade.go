package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/plugin"
	"github.com/moby/moby/api/types/registry"
)

// PluginUpgrade upgrades a plugin
func (cli *Client) PluginUpgrade(ctx context.Context, name string, options PluginInstallOptions) (io.ReadCloser, error) {
	name, err := trimID("plugin", name)
	if err != nil {
		return nil, err
	}

	if err := cli.NewVersionError(ctx, "1.26", "plugin upgrade"); err != nil {
		return nil, err
	}
	query := url.Values{}
	if _, err := reference.ParseNormalizedNamed(options.RemoteRef); err != nil {
		return nil, fmt.Errorf("invalid remote reference: %w", err)
	}
	query.Set("remote", options.RemoteRef)

	privileges, err := cli.checkPluginPermissions(ctx, query, options)
	if err != nil {
		return nil, err
	}

	resp, err := cli.tryPluginUpgrade(ctx, query, privileges, name, options.RegistryAuth)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (cli *Client) tryPluginUpgrade(ctx context.Context, query url.Values, privileges plugin.Privileges, name, registryAuth string) (*http.Response, error) {
	return cli.post(ctx, "/plugins/"+name+"/upgrade", query, privileges, http.Header{
		registry.AuthHeader: {registryAuth},
	})
}
