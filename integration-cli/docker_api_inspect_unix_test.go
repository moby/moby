// +build !windows

package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

// #16665
func (s *DockerSuite) TestInspectApiCpusetInConfigPre120(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, cgroupCpuset)

	name := "cpusetinconfig-pre120"
	dockerCmd(c, "run", "--name", name, "--cpuset", "0-1", "busybox", "true")

	status, body, err := sockRequest("GET", fmt.Sprintf("/v1.19/containers/%s/json", name), nil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(err, checker.IsNil)

	var inspectJSON map[string]interface{}
	c.Assert(json.Unmarshal(body, &inspectJSON),checker.IsNil,Commentf("unable to unmarshal body for version 1.19: %v",json.Unmarshal(body, &inspectJSON)))

	config, ok := inspectJSON["Config"]
	c.Assert(ok,checker.Equals,true,Commentf("Unable to find 'Config'"))
	cfg := config.(map[string]interface{})
	
	_, ok := cfg["Cpuset"]
	c.Assert(ok,checker.Equals,true,Commentf("Api version 1.19 expected to include Cpuset in 'Config'"))
	
}
