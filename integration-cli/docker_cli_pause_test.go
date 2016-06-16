package main

import (
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

	out, _ := dockerCmd(c, "events", "--since=0", "--until", daemonUnixTime(c))
	events := strings.Split(strings.TrimSpace(out), "\n")
	actions := eventActionsByIDAndType(c, events, name, "container")

	c.Assert(actions[len(actions)-2], checker.Equals, "pause")
	c.Assert(actions[len(actions)-1], checker.Equals, "unpause")
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

	out, _ := dockerCmd(c, "events", "--since=0", "--until", daemonUnixTime(c))
	events := strings.Split(strings.TrimSpace(out), "\n")

	for _, name := range containers {
		actions := eventActionsByIDAndType(c, events, name, "container")

		c.Assert(actions[len(actions)-2], checker.Equals, "pause")
		c.Assert(actions[len(actions)-1], checker.Equals, "unpause")
	}
}

func (s *DockerSuite) TestPauseFailsOnWindows(c *check.C) {
	testRequires(c, DaemonIsWindows)
	dockerCmd(c, "run", "-d", "--name=test", "busybox", "sleep 3")
	out, _, _ := dockerCmdWithError("pause", "test")
	c.Assert(out, checker.Contains, "Windows: Containers cannot be paused")
}
