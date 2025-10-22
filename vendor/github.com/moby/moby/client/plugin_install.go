package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/plugin"
	"github.com/moby/moby/api/types/registry"
)

// PluginInstallOptions holds parameters to install a plugin.
type PluginInstallOptions struct {
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

// PluginInstallResult holds the result of a plugin install operation.
// It is an io.ReadCloser from which the caller can read installation progress or result.
type PluginInstallResult struct {
	io.ReadCloser
}

// PluginInstall installs a plugin
func (cli *Client) PluginInstall(ctx context.Context, name string, options PluginInstallOptions) (_ PluginInstallResult, retErr error) {
	query := url.Values{}
	if _, err := reference.ParseNormalizedNamed(options.RemoteRef); err != nil {
		return PluginInstallResult{}, fmt.Errorf("invalid remote reference: %w", err)
	}
	query.Set("remote", options.RemoteRef)

	privileges, err := cli.checkPluginPermissions(ctx, query, &options)
	if err != nil {
		return PluginInstallResult{}, err
	}

	// set name for plugin pull, if empty should default to remote reference
	query.Set("name", name)

	resp, err := cli.tryPluginPull(ctx, query, privileges, options.RegistryAuth)
	if err != nil {
		return PluginInstallResult{}, err
	}

	name = resp.Header.Get("Docker-Plugin-Name")

	pr, pw := io.Pipe()
	go func() { // todo: the client should probably be designed more around the actual api
		_, err := io.Copy(pw, resp.Body)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		defer func() {
			if retErr != nil {
				delResp, _ := cli.delete(ctx, "/plugins/"+name, nil, nil)
				ensureReaderClosed(delResp)
			}
		}()
		if len(options.Args) > 0 {
			if _, err := cli.PluginSet(ctx, name, PluginSetOptions{Args: options.Args}); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}

		if options.Disabled {
			_ = pw.Close()
			return
		}

		_, enableErr := cli.PluginEnable(ctx, name, PluginEnableOptions{Timeout: 0})
		_ = pw.CloseWithError(enableErr)
	}()
	return PluginInstallResult{pr}, nil
}

func (cli *Client) tryPluginPrivileges(ctx context.Context, query url.Values, registryAuth string) (*http.Response, error) {
	return cli.get(ctx, "/plugins/privileges", query, http.Header{
		registry.AuthHeader: {registryAuth},
	})
}

func (cli *Client) tryPluginPull(ctx context.Context, query url.Values, privileges plugin.Privileges, registryAuth string) (*http.Response, error) {
	return cli.post(ctx, "/plugins/pull", query, privileges, http.Header{
		registry.AuthHeader: {registryAuth},
	})
}

func (cli *Client) checkPluginPermissions(ctx context.Context, query url.Values, options pluginOptions) (plugin.Privileges, error) {
	resp, err := cli.tryPluginPrivileges(ctx, query, options.getRegistryAuth())
	if cerrdefs.IsUnauthorized(err) && options.getPrivilegeFunc() != nil {
		// TODO: do inspect before to check existing name before checking privileges
		newAuthHeader, privilegeErr := options.getPrivilegeFunc()(ctx)
		if privilegeErr != nil {
			ensureReaderClosed(resp)
			return nil, privilegeErr
		}
		options.setRegistryAuth(newAuthHeader)
		resp, err = cli.tryPluginPrivileges(ctx, query, options.getRegistryAuth())
	}
	if err != nil {
		ensureReaderClosed(resp)
		return nil, err
	}

	var privileges plugin.Privileges
	if err := json.NewDecoder(resp.Body).Decode(&privileges); err != nil {
		ensureReaderClosed(resp)
		return nil, err
	}
	ensureReaderClosed(resp)

	if !options.getAcceptAllPermissions() && options.getAcceptPermissionsFunc() != nil && len(privileges) > 0 {
		accept, err := options.getAcceptPermissionsFunc()(ctx, privileges)
		if err != nil {
			return nil, err
		}
		if !accept {
			return nil, errors.New("permission denied while installing plugin " + options.getRemoteRef())
		}
	}
	return privileges, nil
}

type pluginOptions interface {
	getRegistryAuth() string
	setRegistryAuth(string)
	getPrivilegeFunc() func(context.Context) (string, error)
	getAcceptAllPermissions() bool
	getAcceptPermissionsFunc() func(context.Context, plugin.Privileges) (bool, error)
	getRemoteRef() string
}

func (o *PluginInstallOptions) getRegistryAuth() string {
	return o.RegistryAuth
}

func (o *PluginInstallOptions) setRegistryAuth(auth string) {
	o.RegistryAuth = auth
}

func (o *PluginInstallOptions) getPrivilegeFunc() func(context.Context) (string, error) {
	return o.PrivilegeFunc
}

func (o *PluginInstallOptions) getAcceptAllPermissions() bool {
	return o.AcceptAllPermissions
}

func (o *PluginInstallOptions) getAcceptPermissionsFunc() func(context.Context, plugin.Privileges) (bool, error) {
	return o.AcceptPermissionsFunc
}

func (o *PluginInstallOptions) getRemoteRef() string {
	return o.RemoteRef
}
