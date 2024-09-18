package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func (s *DockerAPISuite) TestInspectAPIContainerResponse(c *testing.T) {
	out := cli.DockerCmd(c, "run", "-d", "busybox", "true").Stdout()
	cleanedContainerID := strings.TrimSpace(out)

	keysBase := []string{
		"Id", "State", "Created", "Path", "Args", "Config", "Image", "NetworkSettings",
		"ResolvConfPath", "HostnamePath", "HostsPath", "LogPath", "Name", "Driver", "MountLabel", "ProcessLabel", "GraphDriver",
		"Mounts",
	}

	cases := []struct {
		version string
		keys    []string
	}{
		{version: "v1.24", keys: keysBase},
	}
	for _, cs := range cases {
		body := getInspectBody(c, cs.version, cleanedContainerID)

		var inspectJSON map[string]interface{}
		err := json.Unmarshal(body, &inspectJSON)
		assert.NilError(c, err, "Unable to unmarshal body for version %s", cs.version)

		for _, key := range cs.keys {
			_, ok := inspectJSON[key]
			assert.Check(c, ok, "%s does not exist in response for version %s", key, cs.version)
		}

		// Issue #6830: type not properly converted to JSON/back
		_, ok := inspectJSON["Path"].(bool)
		assert.Assert(c, !ok, "Path of `true` should not be converted to boolean `true` via JSON marshalling")
	}
}

func (s *DockerAPISuite) TestInspectAPIContainerVolumeDriver(c *testing.T) {
	out := cli.DockerCmd(c, "run", "-d", "--volume-driver", "local", "busybox", "true").Stdout()
	cleanedContainerID := strings.TrimSpace(out)

	body := getInspectBody(c, "v1.25", cleanedContainerID)

	var inspectJSON map[string]interface{}
	err := json.Unmarshal(body, &inspectJSON)
	assert.NilError(c, err, "Unable to unmarshal body for version 1.25")

	config, ok := inspectJSON["Config"]
	assert.Assert(c, ok, "Unable to find 'Config'")
	cfg := config.(map[string]interface{})
	_, ok = cfg["VolumeDriver"]
	assert.Assert(c, !ok, "API version 1.25 expected to not include VolumeDriver in 'Config'")

	config, ok = inspectJSON["HostConfig"]
	assert.Assert(c, ok, "Unable to find 'HostConfig'")
	cfg = config.(map[string]interface{})
	_, ok = cfg["VolumeDriver"]
	assert.Assert(c, ok, "API version 1.25 expected to include VolumeDriver in 'HostConfig'")
}

func (s *DockerAPISuite) TestInspectAPIImageResponse(c *testing.T) {
	cli.DockerCmd(c, "tag", "busybox:latest", "busybox:mytag")
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	imageJSON, _, err := apiClient.ImageInspectWithRaw(testutil.GetContext(c), "busybox")
	assert.NilError(c, err)

	assert.Check(c, len(imageJSON.RepoTags) == 2)
	assert.Check(c, is.Contains(imageJSON.RepoTags, "busybox:latest"))
	assert.Check(c, is.Contains(imageJSON.RepoTags, "busybox:mytag"))
}

// Inspect for API v1.21 and up; see
//
// - https://github.com/moby/moby/issues/17131
// - https://github.com/moby/moby/issues/17139
// - https://github.com/moby/moby/issues/17173
func (s *DockerAPISuite) TestInspectAPIBridgeNetworkSettings121(c *testing.T) {
	// Windows doesn't have any bridge network settings
	testRequires(c, DaemonIsLinux)
	out := cli.DockerCmd(c, "run", "-d", "busybox", "top").Stdout()
	containerID := strings.TrimSpace(out)
	cli.WaitRun(c, containerID)

	body := getInspectBody(c, "", containerID)

	var inspectJSON container.InspectResponse
	err := json.Unmarshal(body, &inspectJSON)
	assert.NilError(c, err)

	settings := inspectJSON.NetworkSettings
	assert.Assert(c, len(settings.IPAddress) != 0)
	assert.Assert(c, settings.Networks["bridge"] != nil)
	assert.Equal(c, settings.IPAddress, settings.Networks["bridge"].IPAddress)
}
