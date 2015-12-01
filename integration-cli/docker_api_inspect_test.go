package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions/v1p20"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestInspectApiContainerResponse(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")

	cleanedContainerID := strings.TrimSpace(out)
	keysBase := []string{"Id", "State", "Created", "Path", "Args", "Config", "Image", "NetworkSettings",
		"ResolvConfPath", "HostnamePath", "HostsPath", "LogPath", "Name", "Driver", "MountLabel", "ProcessLabel", "GraphDriver"}

	cases := []struct {
		version string
		keys    []string
	}{
		{"v1.20", append(keysBase, "Mounts")},
		{"v1.19", append(keysBase, "Volumes", "VolumesRW")},
	}

	for _, cs := range cases {
		body := getInspectBody(c, cs.version, cleanedContainerID)

		var inspectJSON map[string]interface{}
		err := json.Unmarshal(body, &inspectJSON)
		c.Assert(err, checker.IsNil, check.Commentf("Unable to unmarshal body for version %s", cs.version))

		for _, key := range cs.keys {
			_, ok := inspectJSON[key]
			c.Check(ok, checker.True, check.Commentf("%s does not exist in response for version %s", key, cs.version))
		}

		//Issue #6830: type not properly converted to JSON/back
		_, ok := inspectJSON["Path"].(bool)
		c.Assert(ok, checker.False, check.Commentf("Path of `true` should not be converted to boolean `true` via JSON marshalling"))
	}
}

func (s *DockerSuite) TestInspectApiContainerVolumeDriverLegacy(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")

	cleanedContainerID := strings.TrimSpace(out)

	cases := []string{"v1.19", "v1.20"}
	for _, version := range cases {
		body := getInspectBody(c, version, cleanedContainerID)

		var inspectJSON map[string]interface{}
		err := json.Unmarshal(body, &inspectJSON)
		c.Assert(err, checker.IsNil, check.Commentf("Unable to unmarshal body for version %s", version))

		config, ok := inspectJSON["Config"]
		c.Assert(ok, checker.True, check.Commentf("Unable to find 'Config'"))
		cfg := config.(map[string]interface{})
		_, ok = cfg["VolumeDriver"]
		c.Assert(ok, checker.True, check.Commentf("Api version %s expected to include VolumeDriver in 'Config'", version))
	}
}

func (s *DockerSuite) TestInspectApiContainerVolumeDriver(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")

	cleanedContainerID := strings.TrimSpace(out)

	body := getInspectBody(c, "v1.21", cleanedContainerID)

	var inspectJSON map[string]interface{}
	err := json.Unmarshal(body, &inspectJSON)
	c.Assert(err, checker.IsNil, check.Commentf("Unable to unmarshal body for version 1.21"))

	config, ok := inspectJSON["Config"]
	c.Assert(ok, checker.True, check.Commentf("Unable to find 'Config'"))
	cfg := config.(map[string]interface{})
	_, ok = cfg["VolumeDriver"]
	c.Assert(ok, checker.False, check.Commentf("Api version 1.21 expected to not include VolumeDriver in 'Config'"))

	config, ok = inspectJSON["HostConfig"]
	c.Assert(ok, checker.True, check.Commentf("Unable to find 'Config'"))
	cfg = config.(map[string]interface{})
	_, ok = cfg["VolumeDriver"]
	c.Assert(ok, checker.True, check.Commentf("Api version 1.21 expected to include VolumeDriver in 'HostConfig'"))
}

func (s *DockerSuite) TestInspectApiImageResponse(c *check.C) {
	dockerCmd(c, "tag", "busybox:latest", "busybox:mytag")

	endpoint := "/images/busybox/json"
	status, body, err := sockRequest("GET", endpoint, nil)

	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var imageJSON types.ImageInspect
	err = json.Unmarshal(body, &imageJSON)
	c.Assert(err, checker.IsNil, check.Commentf("Unable to unmarshal body for latest version"))
	c.Assert(imageJSON.RepoTags, checker.HasLen, 2)

	c.Assert(stringutils.InSlice(imageJSON.RepoTags, "busybox:latest"), checker.Equals, true)
	c.Assert(stringutils.InSlice(imageJSON.RepoTags, "busybox:mytag"), checker.Equals, true)
}

// #17131, #17139, #17173
func (s *DockerSuite) TestInspectApiEmptyFieldsInConfigPre121(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")

	cleanedContainerID := strings.TrimSpace(out)

	cases := []string{"v1.19", "v1.20"}
	for _, version := range cases {
		body := getInspectBody(c, version, cleanedContainerID)

		var inspectJSON map[string]interface{}
		err := json.Unmarshal(body, &inspectJSON)
		c.Assert(err, checker.IsNil, check.Commentf("Unable to unmarshal body for version %s", version))
		config, ok := inspectJSON["Config"]
		c.Assert(ok, checker.True, check.Commentf("Unable to find 'Config'"))
		cfg := config.(map[string]interface{})
		for _, f := range []string{"MacAddress", "NetworkDisabled", "ExposedPorts"} {
			_, ok := cfg[f]
			c.Check(ok, checker.True, check.Commentf("Api version %s expected to include %s in 'Config'", version, f))
		}
	}
}

func (s *DockerSuite) TestInspectApiBridgeNetworkSettings120(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	containerID := strings.TrimSpace(out)
	waitRun(containerID)

	body := getInspectBody(c, "v1.20", containerID)

	var inspectJSON v1p20.ContainerJSON
	err := json.Unmarshal(body, &inspectJSON)
	c.Assert(err, checker.IsNil)

	settings := inspectJSON.NetworkSettings
	c.Assert(settings.IPAddress, checker.Not(checker.HasLen), 0)
}

func (s *DockerSuite) TestInspectApiBridgeNetworkSettings121(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	containerID := strings.TrimSpace(out)
	waitRun(containerID)

	body := getInspectBody(c, "v1.21", containerID)

	var inspectJSON types.ContainerJSON
	err := json.Unmarshal(body, &inspectJSON)
	c.Assert(err, checker.IsNil)

	settings := inspectJSON.NetworkSettings
	c.Assert(settings.IPAddress, checker.Not(checker.HasLen), 0)
	c.Assert(settings.Networks["bridge"], checker.Not(checker.IsNil))
	c.Assert(settings.IPAddress, checker.Equals, settings.Networks["bridge"].IPAddress)
}
