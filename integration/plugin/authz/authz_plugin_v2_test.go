// +build !windows

package authz

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/integration/internal/api/container"
	"github.com/docker/docker/integration/internal/api/plugin"
	"github.com/docker/docker/integration/internal/api/volume"
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
	requirement.HasHubConnectivity(t)

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
	err = plugin.InstallGrantAllPermissions(client, authzPluginNameWithTag)
	require.Nil(t, err)
	// start the daemon with the plugin and load busybox, --net=none build fails otherwise
	// because it needs to pull busybox
	d.Restart(t, "--authorization-plugin="+authzPluginNameWithTag)
	d.LoadBusybox(t)

	// Ensure docker run command and accompanying docker ps are successful
	id, err := container.Run(client, "busybox", []string{"top"})
	require.Nil(t, err)

	_, err = client.ContainerInspect(context.Background(), id)
	require.Nil(t, err)
}

func TestAuthZPluginV2Disable(t *testing.T) {
	skip.IfCondition(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTestV2(t)()

	client, err := d.NewClient()
	require.Nil(t, err)

	// Install authz plugin
	err = plugin.InstallGrantAllPermissions(client, authzPluginNameWithTag)
	require.Nil(t, err)

	d.Restart(t, "--authorization-plugin="+authzPluginNameWithTag)
	d.LoadBusybox(t)

	_, err = volume.Create(client, "local", map[string]string{})
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	// disable the plugin
	err = plugin.Disable(client, authzPluginNameWithTag)
	require.Nil(t, err)

	// now test to see if the docker api works.
	_, err = volume.Create(client, "local", map[string]string{})
	require.Nil(t, err)
}

func TestAuthZPluginV2RejectVolumeRequests(t *testing.T) {
	skip.IfCondition(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTestV2(t)()

	client, err := d.NewClient()
	require.Nil(t, err)

	// Install authz plugin
	err = plugin.InstallGrantAllPermissions(client, authzPluginNameWithTag)
	require.Nil(t, err)

	// restart the daemon with the plugin
	d.Restart(t, "--authorization-plugin="+authzPluginNameWithTag)

	_, err = volume.Create(client, "local", map[string]string{})
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	_, err = volume.Ls(client)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	// The plugin will block the command before it can determine the volume does not exist
	err = volume.Rm(client, "test")
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	_, err = volume.Inspect(client, "test")
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	_, err = volume.Prune(client)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))
}

func TestAuthZPluginV2BadManifestFailsDaemonStart(t *testing.T) {
	skip.IfCondition(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTestV2(t)()

	client, err := d.NewClient()
	require.Nil(t, err)

	// Install authz plugin with bad manifest
	err = plugin.InstallGrantAllPermissions(client, authzPluginBadManifestName)
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
