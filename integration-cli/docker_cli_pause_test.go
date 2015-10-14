package main

import (
	"fmt"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestPause(c *check.C) {
	testRequires(c, DaemonIsLinux)
	defer unpauseAllContainers()

	name := "testeventpause"
	dockerCmd(c, "run", "-d", "--name", name, "busybox", "top")

	dockerCmd(c, "pause", name)
	pausedContainers, err := getSliceOfPausedContainers()
	c.Assert(err, checker.IsNil)
	c.Assert(len(pausedContainers), checker.Equals, 1)

	dockerCmd(c, "unpause", name)

	out, _ := dockerCmd(c, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(out, "\n")
	c.Assert(len(events) > 1, checker.Equals, true)

	pauseEvent := strings.Fields(events[len(events)-3])
	unpauseEvent := strings.Fields(events[len(events)-2])

	c.Assert(pauseEvent[len(pauseEvent)-1], checker.Equals, "pause")
	c.Assert(unpauseEvent[len(unpauseEvent)-1], checker.Equals, "unpause")

}

func (s *DockerSuite) TestPauseMultipleContainers(c *check.C) {
	testRequires(c, DaemonIsLinux)
	defer unpauseAllContainers()

	containers := []string{
		"testpausewithmorecontainers1",
		"testpausewithmorecontainers2",
	}
	for _, name := range containers {
		dockerCmd(c, "run", "-d", "--name", name, "busybox", "top")
	}
	dockerCmd(c, append([]string{"pause"}, containers...)...)
	pausedContainers, err := getSliceOfPausedContainers()
	c.Assert(err, checker.IsNil)
	c.Assert(len(pausedContainers), checker.Equals, len(containers))

	dockerCmd(c, append([]string{"unpause"}, containers...)...)

	out, _ := dockerCmd(c, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(out, "\n")
	c.Assert(len(events) > len(containers)*3-2, checker.Equals, true)

	pauseEvents := make([][]string, len(containers))
	unpauseEvents := make([][]string, len(containers))
	for i := range containers {
		pauseEvents[i] = strings.Fields(events[len(events)-len(containers)*2-1+i])
		unpauseEvents[i] = strings.Fields(events[len(events)-len(containers)-1+i])
	}

	for _, pauseEvent := range pauseEvents {
		c.Assert(pauseEvent[len(pauseEvent)-1], checker.Equals, "pause")
	}
	for _, unpauseEvent := range unpauseEvents {
		c.Assert(unpauseEvent[len(unpauseEvent)-1], checker.Equals, "unpause")
	}

}
