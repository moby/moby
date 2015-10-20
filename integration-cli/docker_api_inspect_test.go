package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestInspectApiContainerResponse(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")

	cleanedContainerID := strings.TrimSpace(out)
	keysBase := []string{"Id", "State", "Created", "Path", "Args", "Config", "Image", "NetworkSettings",
		"ResolvConfPath", "HostnamePath", "HostsPath", "LogPath", "Name", "Driver", "ExecDriver", "MountLabel", "ProcessLabel", "GraphDriver"}

	cases := []struct {
		version string
		keys    []string
	}{
		{"1.20", append(keysBase, "Mounts")},
		{"1.19", append(keysBase, "Volumes", "VolumesRW")},
	}

	for _, cs := range cases {
		endpoint := fmt.Sprintf("/v%s/containers/%s/json", cs.version, cleanedContainerID)

		status, body, err := sockRequest("GET", endpoint, nil)
		c.Assert(status, checker.Equals, http.StatusOK)
		c.Assert(err, checker.IsNil)

		var inspectJSON map[string]interface{}
		c.Assert(json.Unmarshal(body, &imageJSON), check.IsNil, Commentf("unable to unmarshal body for version %s: %v", cs.version, json.Unmarshal(body, &imageJSON))
		

		for _, key := range cs.keys {
			_, ok := inspectJSON[key]
			c.Assert(ok,checker.Equals,false,Commentf("%s does not exist in response for version %s", key, cs.version))
		}

		//Issue #6830: type not properly converted to JSON/back
		_, ok := inspectJSON["Path"].(bool)
		c.Assert(ok,checker.Equals,false,Commentf("Path of `true` should not be converted to boolean `true` via JSON marshalling"))
		
	}
}

func (s *DockerSuite) TestInspectApiContainerVolumeDriverLegacy(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")

	cleanedContainerID := strings.TrimSpace(out)

	cases := []string{"1.19", "1.20"}
	for _, version := range cases {
		endpoint := fmt.Sprintf("/v%s/containers/%s/json", version, cleanedContainerID)
		status, body, err := sockRequest("GET", endpoint, nil)
		c.Assert(status, checker.Equals, http.StatusOK)
		c.Assert(err, checker.IsNil)

		var inspectJSON map[string]interface{}
		c.Assert(json.Unmarshal(body, &imageJSON), check.IsNil, Commentf("unable to unmarshal body for version %s: %v", version, json.Unmarshal(body, &imageJSON))

		config, ok := inspectJSON["Config"]
		c.Assert(ok,checker.Equals,true,Commentf("Unable to find 'Config'"))

		cfg := config.(map[string]interface{})
		_, ok := cfg["VolumeDriver"]
		c.Assert(ok,checker.Equals,true,Commentf("Api version %s expected to include VolumeDriver in 'Config'", version))
	}
}

func (s *DockerSuite) TestInspectApiContainerVolumeDriver(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")

	cleanedContainerID := strings.TrimSpace(out)

	endpoint := fmt.Sprintf("/v1.21/containers/%s/json", cleanedContainerID)
	status, body, err := sockRequest("GET", endpoint, nil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(err, checker.IsNil)

	var inspectJSON map[string]interface{}
	c.Assert(json.Unmarshal(body, &imageJSON), check.IsNil, Commentf("unable to unmarshal body for version 1.21: %v", err))

	config, ok := inspectJSON["Config"]
	c.Assert(ok,checker.Equals,true,Commentf("Unable to find 'Config'"))

	cfg := config.(map[string]interface{})
	_, ok := cfg["VolumeDriver"]
	c.Assert(ok,checker.Equals,false,Commentf("Api version 1.21 expected to not include VolumeDriver in 'Config'"))

	config, ok = inspectJSON["HostConfig"]
	c.Assert(ok,checker.Equals,true,Commentf("Unable to find 'HostConfig'"))
	cfg = config.(map[string]interface{})
	_, ok := cfg["VolumeDriver"]
	c.Assert(ok,checker.Equals,true,Commentf("Api version 1.21 expected to include VolumeDriver in 'HostConfig'"))
	
}

func (s *DockerSuite) TestInspectApiImageResponse(c *check.C) {
	dockerCmd(c, "tag", "busybox:latest", "busybox:mytag")

	endpoint := "/images/busybox/json"
	status, body, err := sockRequest("GET", endpoint, nil)

	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var imageJSON types.ImageInspect

	c.Assert(json.Unmarshal(body, &imageJSON), check.IsNil, Commentf("unable to unmarshal body for latest version: %v",json.Unmarshal(body, &imageJSON)))

	c.Assert(imageJSON.Tags, checker.hasLen, 2)

	c.Assert(stringutils.InSlice(imageJSON.Tags, "busybox:latest"), checker.Equals, true)
	c.Assert(stringutils.InSlice(imageJSON.Tags, "busybox:mytag"), checker.Equals, true)
}
