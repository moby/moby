// +build !experimental

package main

import (
	"fmt"
	"strings"

	"github.com/docker/docker/daemon/events/testutils"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestEventsImagePull(c *check.C) {
	// TODO Windows: Enable this test once pull and reliable image names are available
	testRequires(c, DaemonIsLinux)
	since := daemonTime(c).Unix()
	testRequires(c, Network)

	dockerCmd(c, "pull", "hello-world")

	out, _ := dockerCmd(c, "events",
		fmt.Sprintf("--since=%d", since),
		fmt.Sprintf("--until=%d", daemonTime(c).Unix()))

	events := strings.Split(strings.TrimSpace(out), "\n")
	event := strings.TrimSpace(events[len(events)-1])
	matches := eventstestutils.ScanMap(event)
	c.Assert(matches["id"], checker.Equals, "hello-world:latest")
	c.Assert(matches["action"], checker.Equals, "pull")

}
