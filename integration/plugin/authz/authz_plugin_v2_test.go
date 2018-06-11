// +build !windows

package authz // import "github.com/docker/docker/integration/plugin/authz"

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/requirement"
	"gotest.tools/assert"
	"gotest.tools/skip"
)

var (
	authzPluginName            = "riyaz/authz-no-volume-plugin"
	authzPluginTag             = "latest"
	authzPluginNameWithTag     = authzPluginName + ":" + authzPluginTag
	authzPluginBadManifestName = "riyaz/authz-plugin-bad-manifest"
	nonexistentAuthzPluginName = "riyaz/nonexistent-authz-plugin"
)

func setupTestV2(t *testing.T) func() {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, !requirement.HasHubConnectivity(t))

	teardown := setupTest(t)

	d.Start(t)

	return teardown
}

func TestAuthZPluginV2AllowNonVolumeRequest(t *testing.T) {
	skip.If(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTestV2(t)()

	client, err := d.NewClient()
	assert.NilError(t, err)

	ctx := context.Background()

	// Install authz plugin
	err = pluginInstallGrantAllPermissions(client, authzPluginNameWithTag)
	assert.NilError(t, err)
	// start the daemon with the plugin and load busybox, --net=none build fails otherwise
	// because it needs to pull busybox
	d.Restart(t, "--authorization-plugin="+authzPluginNameWithTag)
	d.LoadBusybox(t)

	// Ensure docker run command and accompanying docker ps are successful
	cID := container.Run(t, ctx, client)

	_, err = client.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
}

func TestAuthZPluginV2Disable(t *testing.T) {
	skip.If(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTestV2(t)()

	client, err := d.NewClient()
	assert.NilError(t, err)

	// Install authz plugin
	err = pluginInstallGrantAllPermissions(client, authzPluginNameWithTag)
	assert.NilError(t, err)

	d.Restart(t, "--authorization-plugin="+authzPluginNameWithTag)
	d.LoadBusybox(t)

	_, err = client.VolumeCreate(context.Background(), volumetypes.VolumeCreateBody{Driver: "local"})
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	// disable the plugin
	err = client.PluginDisable(context.Background(), authzPluginNameWithTag, types.PluginDisableOptions{})
	assert.NilError(t, err)

	// now test to see if the docker api works.
	_, err = client.VolumeCreate(context.Background(), volumetypes.VolumeCreateBody{Driver: "local"})
	assert.NilError(t, err)
}

func TestAuthZPluginV2RejectVolumeRequests(t *testing.T) {
	skip.If(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTestV2(t)()

	client, err := d.NewClient()
	assert.NilError(t, err)

	// Install authz plugin
	err = pluginInstallGrantAllPermissions(client, authzPluginNameWithTag)
	assert.NilError(t, err)

	// restart the daemon with the plugin
	d.Restart(t, "--authorization-plugin="+authzPluginNameWithTag)

	_, err = client.VolumeCreate(context.Background(), volumetypes.VolumeCreateBody{Driver: "local"})
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	_, err = client.VolumeList(context.Background(), filters.Args{})
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	// The plugin will block the command before it can determine the volume does not exist
	err = client.VolumeRemove(context.Background(), "test", false)
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	_, err = client.VolumeInspect(context.Background(), "test")
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))

	_, err = client.VolumesPrune(context.Background(), filters.Args{})
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), fmt.Sprintf("Error response from daemon: plugin %s failed with error:", authzPluginNameWithTag)))
}

func TestAuthZPluginV2BadManifestFailsDaemonStart(t *testing.T) {
	skip.If(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTestV2(t)()

	client, err := d.NewClient()
	assert.NilError(t, err)

	// Install authz plugin with bad manifest
	err = pluginInstallGrantAllPermissions(client, authzPluginBadManifestName)
	assert.NilError(t, err)

	// start the daemon with the plugin, it will error
	err = d.RestartWithError("--authorization-plugin=" + authzPluginBadManifestName)
	assert.Assert(t, err != nil)

	// restarting the daemon without requiring the plugin will succeed
	d.Start(t)
}

func TestAuthZPluginV2NonexistentFailsDaemonStart(t *testing.T) {
	defer setupTestV2(t)()

	// start the daemon with a non-existent authz plugin, it will error
	err := d.RestartWithError("--authorization-plugin=" + nonexistentAuthzPluginName)
	assert.Assert(t, err != nil)

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
