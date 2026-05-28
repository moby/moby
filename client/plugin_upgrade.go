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

// PluginUpgradeOptions holds parameters to upgrade a plugin.
type PluginUpgradeOptions struct {
	Disabled             bool
	AcceptAllPermissions bool
	RegistryAuth         string // RegistryAuth is the base64 encoded credentials for the registry
	RemoteRef            string // RemoteRef is the plugin name on the registry

	// PrivilegeFunc is a function that clients can supply to retry operations
	// after getting an authorization error. This function returns the registry
	// authentication header value in base64 encoded format, or an error if the
	// privilege request fails.
	//
	// For details, refer to [github.com/moby/moby/api/types/registry.RequestAuthConfig].
	PrivilegeFunc         func(context.Context) (string, error)
	AcceptPermissionsFunc func(context.Context, plugin.Privileges) (bool, error)
	Args                  []string
}

// PluginUpgradeResult holds the result of a plugin upgrade operation.
type PluginUpgradeResult io.ReadCloser

// PluginUpgrade upgrades a plugin
func (cli *Client) PluginUpgrade(ctx context.Context, name string, options PluginUpgradeOptions) (PluginUpgradeResult, error) {
	name, err := trimID("plugin", name)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	if _, err := reference.ParseNormalizedNamed(options.RemoteRef); err != nil {
		return nil, fmt.Errorf("invalid remote reference: %w", err)
	}
	query.Set("remote", options.RemoteRef)

	privileges, err := cli.checkPluginPermissions(ctx, query, &options)
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

func (o *PluginUpgradeOptions) getRegistryAuth() string {
	return o.RegistryAuth
}

func (o *PluginUpgradeOptions) setRegistryAuth(auth string) {
	o.RegistryAuth = auth
}

func (o *PluginUpgradeOptions) getPrivilegeFunc() func(context.Context) (string, error) {
	return o.PrivilegeFunc
}

func (o *PluginUpgradeOptions) getAcceptAllPermissions() bool {
	return o.AcceptAllPermissions
}

func (o *PluginUpgradeOptions) getAcceptPermissionsFunc() func(context.Context, plugin.Privileges) (bool, error) {
	return o.AcceptPermissionsFunc
}

func (o *PluginUpgradeOptions) getRemoteRef() string {
	return o.RemoteRef
}
