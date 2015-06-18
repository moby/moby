package main

import (
	"encoding/json"
	"net/http"
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

	endpoint := "/containers/" + cleanedContainerID + "/json"
	status, body, err := sockRequest("GET", endpoint, nil)
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)

	var inspectJSON map[string]interface{}
	if err = json.Unmarshal(body, &inspectJSON); err != nil {
		c.Fatalf("unable to unmarshal body for latest version: %v", err)
	}

	keys := []string{"State", "Created", "Path", "Args", "Config", "Image", "NetworkSettings", "ResolvConfPath", "HostnamePath", "HostsPath", "LogPath", "Name", "Driver", "ExecDriver", "MountLabel", "ProcessLabel", "Volumes", "VolumesRW"}

	keys = append(keys, "Id")

	for _, key := range keys {
		if _, ok := inspectJSON[key]; !ok {
			c.Fatalf("%s does not exist in response for latest version", key)
		}
	}
	//Issue #6830: type not properly converted to JSON/back
	if _, ok := inspectJSON["Path"].(bool); ok {
		c.Fatalf("Path of `true` should not be converted to boolean `true` via JSON marshalling")
	}
}
