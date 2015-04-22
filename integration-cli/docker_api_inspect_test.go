package main

import (
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestInspectApiContainerResponse(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("failed to create a container: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	// test on json marshal version
	// and latest version
	testVersions := []string{"v1.11", "latest"}

	for _, testVersion := range testVersions {
		endpoint := "/containers/" + cleanedContainerID + "/json"
		if testVersion != "latest" {
			endpoint = "/" + testVersion + endpoint
		}
		_, body, err := sockRequest("GET", endpoint, nil)
		if err != nil {
			c.Fatalf("sockRequest failed for %s version: %v", testVersion, err)
		}

		var inspectJSON map[string]interface{}
		if err = json.Unmarshal(body, &inspectJSON); err != nil {
			c.Fatalf("unable to unmarshal body for %s version: %v", testVersion, err)
		}

		keys := []string{"State", "Created", "Path", "Args", "Config", "Image", "NetworkSettings", "ResolvConfPath", "HostnamePath", "HostsPath", "LogPath", "Name", "Driver", "ExecDriver", "MountLabel", "ProcessLabel", "Volumes", "VolumesRW"}

		if testVersion == "v1.11" {
			keys = append(keys, "ID")
		} else {
			keys = append(keys, "Id")
		}

		for _, key := range keys {
			if _, ok := inspectJSON[key]; !ok {
				c.Fatalf("%s does not exist in response for %s version", key, testVersion)
			}
		}
		//Issue #6830: type not properly converted to JSON/back
		if _, ok := inspectJSON["Path"].(bool); ok {
			c.Fatalf("Path of `true` should not be converted to boolean `true` via JSON marshalling")
		}
	}
}
