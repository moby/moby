// +build !windows

package authz

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	networktypes "github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/util/requirement"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	authzPluginName            = "riyaz/authz-no-volume-plugin"
	authzPluginAlias           = "authz-no-volume"
	authzPluginTag             = "latest"
	authzPluginNameWithTag     = authzPluginName + ":" + authzPluginTag
	authzPluginAliasWithTag    = authzPluginAlias + ":" + authzPluginTag
	authzPluginBadManifestName = "riyaz/authz-plugin-bad-manifest"
	nonexistentAuthzPluginName = "riyaz/nonexistent-authz-plugin"
	authzPluginV1Alias         = "authzplugin2:latest"
)

func setupTestV2(t *testing.T) func() {
	skip.IfCondition(t, testEnv.DaemonInfo.OSType != "linux")
	skip.IfCondition(t, !requirement.HasHubConnectivity(t))

	teardown := setupTest(t)

	d.Start(t)

	return teardown
}

func TestAuthZPluginV2AllowNonVolumeRequest(t *testing.T) {
	skip.IfCondition(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTestV2(t)()

	client, err := d.NewClient()
	require.Nil(t, err)

	// Install authz plugin
	err = pluginInstallGrantAllPermissionsAlias(client, "", authzPluginNameWithTag)
	require.Nil(t, err)
	// start the daemon with the plugin and load busybox, --net=none build fails otherwise
	// because it needs to pull busybox
	d.Restart(t, "--authorization-plugin="+authzPluginNameWithTag)
	d.LoadBusybox(t)

	// Ensure docker run command and accompanying docker ps are successful
	createResponse, err := client.ContainerCreate(context.Background(), &container.Config{Cmd: []string{"top"}, Image: "busybox"}, &container.HostConfig{}, &networktypes.NetworkingConfig{}, "")
	require.Nil(t, err)

	err = client.ContainerStart(context.Background(), createResponse.ID, types.ContainerStartOptions{})
	require.Nil(t, err)

	_, err = client.ContainerInspect(context.Background(), createResponse.ID)
	require.Nil(t, err)
}

func TestAuthZPluginV2Disable(t *testing.T) {
	skip.IfCondition(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTestV2(t)()

	client, err := d.NewClient()
	require.Nil(t, err)

	// Install authz plugin
	err = pluginInstallGrantAllPermissionsAlias(client, "", authzPluginNameWithTag)
	require.Nil(t, err)

	_, err = client.VolumeCreate(context.Background(), volumetypes.VolumesCreateBody{Driver: "local"})
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	d.Restart(t)
	d.LoadBusybox(t)

	_, err = client.VolumeCreate(context.Background(), volumetypes.VolumesCreateBody{Driver: "local"})
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	d.Restart(t, "--authorization-plugin="+authzPluginNameWithTag)

	_, err = client.VolumeCreate(context.Background(), volumetypes.VolumesCreateBody{Driver: "local"})
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	// disable the plugin
	err = client.PluginDisable(context.Background(), authzPluginNameWithTag, types.PluginDisableOptions{})
	require.Nil(t, err)

	// now test to see if the docker api works.
	_, err = client.VolumeCreate(context.Background(), volumetypes.VolumesCreateBody{Driver: "local"})
	require.Nil(t, err)

	// re-enable the plugin
	err = client.PluginEnable(context.Background(), authzPluginNameWithTag, types.PluginEnableOptions{})
	require.Nil(t, err)

	_, err = client.VolumeCreate(context.Background(), volumetypes.VolumesCreateBody{Driver: "local"})
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))
}

func TestAuthZPluginV2RejectVolumeRequests(t *testing.T) {
	skip.IfCondition(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTestV2(t)()

	client, err := d.NewClient()
	require.Nil(t, err)

	// Install authz plugin
	err = pluginInstallGrantAllPermissionsAlias(client, "", authzPluginNameWithTag)
	require.Nil(t, err)

	// restart the daemon with the plugin
	d.Restart(t, "--authorization-plugin="+authzPluginNameWithTag)

	_, err = client.VolumeCreate(context.Background(), volumetypes.VolumesCreateBody{Driver: "local"})
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	_, err = client.VolumeList(context.Background(), filters.Args{})
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	// The plugin will block the command before it can determine the volume does not exist
	err = client.VolumeRemove(context.Background(), "test", false)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	_, err = client.VolumeInspect(context.Background(), "test")
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	_, err = client.VolumesPrune(context.Background(), filters.Args{})
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))
}

func TestAuthZPluginV2BadManifestFailsDaemonStart(t *testing.T) {
	skip.IfCondition(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTestV2(t)()

	client, err := d.NewClient()
	require.Nil(t, err)

	// Install authz plugin with bad manifest
	err = pluginInstallGrantAllPermissionsAlias(client, "", authzPluginBadManifestName)
	require.Nil(t, err)

	// start the daemon with the plugin, it will error
	err = d.RestartWithError("--authorization-plugin=" + authzPluginBadManifestName)
	require.NotNil(t, err)

	// restarting the daemon without requiring the plugin will succeed
	d.Start(t)
}

func TestAuthZPluginV2NonexistentFailsDaemonStart(t *testing.T) {
	defer setupTestV2(t)()

	// start the daemon with a non-existent authz plugin, it will error
	err := d.RestartWithError("--authorization-plugin=" + nonexistentAuthzPluginName)
	require.NotNil(t, err)

	// restarting the daemon without requiring the plugin will succeed
	d.Start(t)
}

func TestAuthZPluginV2TagAlias(t *testing.T) {
	defer setupTestV2(t)()

	client, err := d.NewClient()
	require.Nil(t, err)

	err = pluginInstallGrantAllPermissionsAlias(client, "", authzPluginName)
	require.Nil(t, err)

	// after single install
	assertAuthzChainSequence(t, client, []string{authzPluginNameWithTag})

	d.Restart(t, "--authorization-plugin="+authzPluginName, "--authorization-plugin="+authzPluginNameWithTag)

	// after daemon CLI tag+tagless alias specification
	assertAuthzChainSequence(t, client, []string{authzPluginNameWithTag})
}

func TestAuthZPluginV2CombinedV1(t *testing.T) {
	skip.IfCondition(t, testEnv.DaemonInfo.OSType != "linux")
	requirement.HasHubConnectivity(t)

	defer setupTestV1(t)()
	ctrl.reqRes.Allow = true
	ctrl.resRes.Allow = true

	// create a second V1 plugin that points to the same plugin process
	fileName := fmt.Sprintf("/etc/docker/plugins/%s.spec", authzPluginV1Alias)
	err := ioutil.WriteFile(fileName, []byte(server.URL), 0644)
	require.Nil(t, err)

	d.Start(t, "--authorization-plugin="+authzPluginV1Alias, "--authorization-plugin="+testAuthZPlugin)

	client, err := d.NewClient()
	require.Nil(t, err)

	// multiple CLI plugins
	assertAuthzChainSequence(t, client, []string{authzPluginV1Alias, testAuthZPlugin})

	err = client.PluginDisable(context.Background(), authzPluginV1Alias, types.PluginDisableOptions{})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("Error response from daemon: plugin \"%s\" not found", authzPluginV1Alias))

	// disable CLI plugin
	assertAuthzChainSequence(t, client, []string{authzPluginV1Alias, testAuthZPlugin})

	err = pluginInstallGrantAllPermissionsAlias(client, authzPluginV1Alias, authzPluginNameWithTag)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("Error response from daemon: v1 plugin %s already exists in authz chain, cannot add v2 plugin", authzPluginV1Alias))

	// plugin alias same as CLI plugin
	assertAuthzChainSequence(t, client, []string{authzPluginV1Alias, testAuthZPlugin})

	err = client.PluginDisable(context.Background(), authzPluginV1Alias, types.PluginDisableOptions{})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("Error response from daemon: plugin is already disabled: plugin %s found but disabled", authzPluginV1Alias))

	// disable alias plugin
	assertAuthzChainSequence(t, client, []string{authzPluginV1Alias, testAuthZPlugin})
}

func TestAuthZPluginV2ChainSequence(t *testing.T) {
	defer setupTestV2(t)()

	client, err := d.NewClient()
	require.Nil(t, err)

	err = pluginInstallGrantAllPermissionsAlias(client, "", authzPluginNameWithTag)
	require.Nil(t, err)

	// after single install
	assertAuthzChainSequence(t, client, []string{authzPluginNameWithTag})

	d.Restart(t, "--authorization-plugin="+authzPluginNameWithTag)

	// after daemon command line specification
	assertAuthzChainSequence(t, client, []string{authzPluginNameWithTag})

	err = pluginInstallGrantAllPermissionsAlias(client, authzPluginAlias, authzPluginNameWithTag)
	require.Nil(t, err)

	// after alias install
	assertAuthzChainSequence(t, client, []string{authzPluginNameWithTag, authzPluginAliasWithTag})

	d.Restart(t)

	// after restart with 2 plugins enabled
	assertAuthzChainSequence(t, client, []string{authzPluginNameWithTag, authzPluginAliasWithTag})

	d.Restart(t, "--authorization-plugin="+authzPluginAliasWithTag)

	// after restart with 2 plugins enabled (alias specified on daemon CLI)
	assertAuthzChainSequence(t, client, []string{authzPluginAliasWithTag, authzPluginNameWithTag})

	err = client.PluginDisable(context.Background(), authzPluginAlias, types.PluginDisableOptions{})
	require.Nil(t, err)

	// after disabling alias specified on daemon CLI
	assertAuthzChainSequence(t, client, []string{authzPluginNameWithTag})

	err = client.PluginEnable(context.Background(), authzPluginAlias, types.PluginEnableOptions{})
	require.Nil(t, err)

	// after alias re-enable
	assertAuthzChainSequence(t, client, []string{authzPluginNameWithTag, authzPluginAliasWithTag})

	d.Restart(t, "--authorization-plugin="+authzPluginAliasWithTag, "--authorization-plugin="+authzPluginNameWithTag)

	// after both plugins specified in daemon CLI
	assertAuthzChainSequence(t, client, []string{authzPluginAliasWithTag, authzPluginNameWithTag})
}

func pluginInstallGrantAllPermissionsAlias(client client.APIClient, alias, name string) error {
	ctx := context.Background()
	options := types.PluginInstallOptions{
		RemoteRef:            name,
		AcceptAllPermissions: true,
	}
	responseReader, err := client.PluginInstall(ctx, alias, options)
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

func assertAuthzChainSequence(t *testing.T, client client.APIClient, chain []string) {
	info, err := client.Info(context.Background())
	require.Nil(t, err)

	assert.Equal(t, chain, info.Plugins.Authorization)
}
