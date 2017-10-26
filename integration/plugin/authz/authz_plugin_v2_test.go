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
	"github.com/stretchr/testify/require"
)

var (
	authzPluginName            = "riyaz/authz-no-volume-plugin"
	authzPluginTag             = "latest"
	authzPluginNameWithTag     = authzPluginName + ":" + authzPluginTag
	authzPluginBadManifestName = "riyaz/authz-plugin-bad-manifest"
	nonexistentAuthzPluginName = "riyaz/nonexistent-authz-plugin"
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
	err = pluginInstallGrantAllPermissions(client, authzPluginNameWithTag)
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
	err = pluginInstallGrantAllPermissions(client, authzPluginNameWithTag)
	require.Nil(t, err)

	d.Restart(t, "--authorization-plugin="+authzPluginNameWithTag)
	d.LoadBusybox(t)

	_, err = client.VolumeCreate(context.Background(), volumetypes.VolumesCreateBody{Driver: "local"})
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	// disable the plugin
	err = client.PluginDisable(context.Background(), authzPluginNameWithTag, types.PluginDisableOptions{})
	require.Nil(t, err)

	// now test to see if the docker api works.
	_, err = client.VolumeCreate(context.Background(), volumetypes.VolumesCreateBody{Driver: "local"})
	require.Nil(t, err)
}

func TestAuthZPluginV2RejectVolumeRequests(t *testing.T) {
	skip.IfCondition(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTestV2(t)()

	client, err := d.NewClient()
	require.Nil(t, err)

	// Install authz plugin
	err = pluginInstallGrantAllPermissions(client, authzPluginNameWithTag)
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
	err = pluginInstallGrantAllPermissions(client, authzPluginBadManifestName)
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

func pluginInstallGrantAllPermissions(client client.APIClient, name string) error {
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
