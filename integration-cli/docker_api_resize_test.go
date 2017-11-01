package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-check/check"
	"github.com/moby/moby/api/types"
	"github.com/moby/moby/client"
	"github.com/moby/moby/integration-cli/checker"
	"github.com/moby/moby/integration-cli/request"
)

func (s *DockerSuite) TestResizeAPIResponse(c *check.C) {
	out := runSleepingContainer(c, "-d")
	cleanedContainerID := strings.TrimSpace(out)
	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	options := types.ResizeOptions{
		Height: 40,
		Width:  40,
	}
	err = cli.ContainerResize(context.Background(), cleanedContainerID, options)
	c.Assert(err, check.IsNil)
}

func (s *DockerSuite) TestResizeAPIHeightWidthNoInt(c *check.C) {
	out := runSleepingContainer(c, "-d")
	cleanedContainerID := strings.TrimSpace(out)

	endpoint := "/containers/" + cleanedContainerID + "/resize?h=foo&w=bar"
	res, _, err := request.Post(endpoint)
	c.Assert(res.StatusCode, check.Equals, http.StatusBadRequest)
	c.Assert(err, check.IsNil)
}

func (s *DockerSuite) TestResizeAPIResponseWhenContainerNotStarted(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	cleanedContainerID := strings.TrimSpace(out)

	// make sure the exited container is not running
	dockerCmd(c, "wait", cleanedContainerID)

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	options := types.ResizeOptions{
		Height: 40,
		Width:  40,
	}

	err = cli.ContainerResize(context.Background(), cleanedContainerID, options)
	c.Assert(err.Error(), checker.Contains, "is not running")
}
