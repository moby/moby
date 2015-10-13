// +build !windows

package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-check/check"
)

// #16665
func (s *DockerSuite) TestInspectApiCpusetInConfigPre120(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, cgroupCpuset)

	name := "cpusetinconfig-pre120"
	dockerCmd(c, "run", "--name", name, "--cpuset", "0-1", "busybox", "true")

	status, body, err := sockRequest("GET", fmt.Sprintf("/v1.19/containers/%s/json", name), nil)
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)

	var inspectJSON map[string]interface{}
	if err = json.Unmarshal(body, &inspectJSON); err != nil {
		c.Fatalf("unable to unmarshal body for version 1.19: %v", err)
	}

	config, ok := inspectJSON["Config"]
	if !ok {
		c.Fatal("Unable to find 'Config'")
	}
	cfg := config.(map[string]interface{})
	if _, ok := cfg["Cpuset"]; !ok {
		c.Fatal("Api version 1.19 expected to include Cpuset in 'Config'")
	}
}
