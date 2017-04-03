package main

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/request"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestPluginLoadSave(c *check.C) {
	testRequires(c, DaemonIsLinux, IsAmd64, Network)
	cli.DockerCmd(c, "plugin", "install", "--grant-all-permissions", pName)
	defer cli.DockerCmd(c, "plugin", "rm", "-f", pName)

	resp, _, err := request.Get("/plugins/save?plugin=" + pName)
	c.Assert(err, checker.IsNil)
	c.Assert(resp.StatusCode, checker.Equals, http.StatusOK)
	defer resp.Body.Close()

	dir, err := ioutil.TempDir("", "plugin-save")
	c.Assert(err, checker.IsNil)
	defer os.RemoveAll(dir)

	pluginPath := filepath.Join(dir, "plugin.tar")
	f, err := os.Create(pluginPath)
	c.Assert(err, checker.IsNil)
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	c.Assert(err, checker.IsNil)

	f, err = os.Open(pluginPath)
	c.Assert(err, checker.IsNil)
	defer f.Close()

	resp, _, err = request.Post("/plugins/load", request.ContentType("application/x-tar"), request.RawContent(f))
	c.Assert(err, checker.IsNil)
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	c.Assert(string(b), checker.Contains, "already exists")

	cli.Docker(cli.Args("plugin", "rm", "-f", pName))

	f, err = os.Open(pluginPath)
	c.Assert(err, checker.IsNil)
	defer f.Close()

	resp, _, err = request.Post("/plugins/load", request.ContentType("application/x-tar"), request.RawContent(f))
	c.Assert(err, checker.IsNil)
	defer resp.Body.Close()
	b, err = ioutil.ReadAll(resp.Body)
	c.Assert(string(b), checker.Contains, "Loaded")
}
