package plugin

import (
	"context"
	"io/ioutil"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// InstallGrantAllPermissions installs the plugin named and grants it
// all permissions it may require
func InstallGrantAllPermissions(client client.APIClient, name string) error {
	ctx := context.Background()
	options := types.PluginInstallOptions{
		RemoteRef:            name,
		AcceptAllPermissions: true,
	}
	responseReader, err := client.PluginInstall(ctx, "", options)
	if err != nil {
		return err
	}
	defer responseReader.Close()
	// we have to read the response out here because the client API
	// actually starts a goroutine which we can only be sure has
	// completed when we get EOF from reading responseBody
	_, err = ioutil.ReadAll(responseReader)
	return err
}

// Enable enables the named plugin
func Enable(client client.APIClient, name string) error {
	ctx := context.Background()
	options := types.PluginEnableOptions{}
	return client.PluginEnable(ctx, name, options)
}

// Disable disables the named plugin
func Disable(client client.APIClient, name string) error {
	ctx := context.Background()
	options := types.PluginDisableOptions{}
	return client.PluginDisable(ctx, name, options)
}

// Rm removes the named plugin
func Rm(client client.APIClient, name string) error {
	return remove(client, name, false)
}

// RmF forces the removal of the named plugin
func RmF(client client.APIClient, name string) error {
	return remove(client, name, true)
}

func remove(client client.APIClient, name string, force bool) error {
	ctx := context.Background()
	options := types.PluginRemoveOptions{Force: force}
	return client.PluginRemove(ctx, name, options)
}
