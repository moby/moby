package main

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestPluginsAPIList(c *check.C) {
	testRequires(c, DaemonIsLinux, ExperimentalDaemon, Network)

	dockerCmd(c, "plugin", "install", "--grant-all-permissions", "--disable", pName)

	status, b, err := sockRequest("GET", "/plugins", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var plugins []types.Plugin
	c.Assert(json.Unmarshal(b, &plugins), checker.IsNil)

	c.Assert(len(plugins), checker.Equals, 1, check.Commentf("\n%v", plugins))
	c.Assert(plugins[0].Enabled, checker.False)
}

func (s *DockerSuite) TestPluginsAPIEnable(c *check.C) {
	testRequires(c, DaemonIsLinux, ExperimentalDaemon, Network)

	dockerCmd(c, "plugin", "install", "--grant-all-permissions", "--disable", pName)

	status, _, err := sockRequest("POST", "/plugins/"+pName+"/enable", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	status, b, err := sockRequest("GET", "/plugins", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var plugins []types.Plugin
	c.Assert(json.Unmarshal(b, &plugins), checker.IsNil)

	c.Assert(len(plugins), checker.Equals, 1, check.Commentf("\n%v", plugins))
	c.Assert(plugins[0].Enabled, checker.True)
}
