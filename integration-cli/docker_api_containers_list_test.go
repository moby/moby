package main

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestContainerApiGetIgnoreLinks(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "links-parent"
	dockerCmd(c, "run", "-d", "--name", name, "busybox", "top")
	dockerCmd(c, "run", "--name", "links-child", "--link", name, "-d", "busybox", "top")

	var err error
	filter := filters.Args{}

	filter, err = filters.ParseFlag("name=links-parent", filter)
	c.Assert(err, check.IsNil)

	param, err := filters.ToParam(filter)
	c.Assert(err, check.IsNil)

	var inspectJSON []struct {
		Names []string
	}

	// Request ignoring links
	status, body, err := sockRequest("GET", "/containers/json?ignoreLinks=1&filters="+param, nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK)

	err = json.Unmarshal(body, &inspectJSON)
	c.Assert(err, check.IsNil)

	c.Assert(inspectJSON, checker.HasLen, 1)
	c.Assert(inspectJSON[0].Names, checker.Not(checker.IsNil))
	c.Assert(inspectJSON[0].Names, checker.HasLen, 1)
	c.Assert(inspectJSON[0].Names[0], checker.Equals, "/links-parent")

	// Request showing links
	status, body, err = sockRequest("GET", "/containers/json?filters="+param, nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK)

	err = json.Unmarshal(body, &inspectJSON)
	c.Assert(err, check.IsNil)

	c.Assert(inspectJSON, checker.HasLen, 1)
	c.Assert(inspectJSON[0].Names, checker.Not(checker.IsNil))
	c.Assert(inspectJSON[0].Names, checker.HasLen, 2)
	c.Assert(inspectJSON[0].Names[0], checker.Equals, "/links-child/links-parent")
	c.Assert(inspectJSON[0].Names[1], checker.Equals, "/links-parent")
}
