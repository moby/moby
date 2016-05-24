package main

import (
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"strconv"
	"strings"
	"time"
)

func waitForStatus(c *check.C, name string, prev string, expected string) {
	prev = prev + "\n"
	expected = expected + "\n"
	for {
		out, _ := dockerCmd(c, "inspect", "--format={{.State.Status}}", name)
		if out == expected {
			return
		}
		c.Check(out, checker.Equals, prev)
		if out != prev {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func waitForHealthStatus(c *check.C, name string, prev string, expected string) {
	prev = prev + "\n"
	expected = expected + "\n"
	for {
		out, _ := dockerCmd(c, "inspect", "--format={{.State.Health.Status}}", name)
		if out == expected {
			return
		}
		c.Check(out, checker.Equals, prev)
		if out != prev {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *DockerSuite) TestHealth(c *check.C) {
	testRequires(c, DaemonIsLinux) // busybox doesn't work on Windows

	imageName := "testhealth"
	_, err := buildImage(imageName,
		`FROM busybox
		RUN echo OK > /status
		CMD ["/bin/sleep", "120"]
		STOPSIGNAL SIGKILL
		HEALTHCHECK --interval=1s --timeout=30s \
		  CMD cat /status`,
		true)

	c.Check(err, check.IsNil)

	// No health status before starting
	name := "test_health"
	dockerCmd(c, "create", "--name", name, imageName)
	out, _ := dockerCmd(c, "ps", "-a", "--format={{.Status}}")
	c.Check(out, checker.Equals, "Created\n")

	// Inspect the options
	out, _ = dockerCmd(c, "inspect",
		"--format='timeout={{.Config.Healthcheck.Timeout}} "+
			"interval={{.Config.Healthcheck.Interval}} "+
			"retries={{.Config.Healthcheck.Retries}} "+
			"test={{.Config.Healthcheck.Test}}'", name)
	c.Check(out, checker.Equals, "timeout=30 interval=1 retries=0 test=[CMD-SHELL cat /status]\n")

	// Start
	dockerCmd(c, "start", name)
	waitForHealthStatus(c, name, "starting", "healthy")

	// Make it fail
	dockerCmd(c, "exec", name, "rm", "/status")
	waitForHealthStatus(c, name, "healthy", "unhealthy")

	// Inspect the status
	out, _ = dockerCmd(c, "inspect", "--format={{.State.Health.Status}}", name)
	c.Check(out, checker.Equals, "unhealthy\n")

	// Make it healthy again
	dockerCmd(c, "exec", name, "touch", "/status")
	waitForHealthStatus(c, name, "unhealthy", "healthy")

	// Remove container
	dockerCmd(c, "rm", "-f", name)

	// Disable the check from the CLI
	out, _ = dockerCmd(c, "create", "--name=noh", "--no-healthcheck", imageName)
	out, _ = dockerCmd(c, "inspect", "--format={{.Config.Healthcheck.Test}}", "noh")
	c.Check(out, checker.Equals, "[NONE]\n")
	dockerCmd(c, "rm", "noh")

	// Disable the check with a new build
	_, err = buildImage("no_healthcheck",
		`FROM testhealth
		HEALTHCHECK NONE`, true)
	c.Check(err, check.IsNil)

	out, _ = dockerCmd(c, "inspect", "--format={{.ContainerConfig.Healthcheck.Test}}", "no_healthcheck")
	c.Check(out, checker.Equals, "[NONE]\n")

	// Enable the checks from the CLI
	_, _ = dockerCmd(c, "run", "-d", "--name=fatal_healthcheck",
		"--health-interval=0.5s",
		"--health-retries=3",
		"--health-cmd=cat /status",
		"no_healthcheck")
	waitForHealthStatus(c, "fatal_healthcheck", "starting", "healthy")
	out, _ = dockerCmd(c, "inspect",
		"--format=status={{.State.Health.Status}} "+
			"fails={{.State.Health.FailingStreak}} "+
			"exit={{.State.Health.LastExitCode}} "+
			"out={{.State.Health.LastOutput}}", "fatal_healthcheck")
	c.Check(strings.TrimSpace(out), checker.Equals, "status=healthy fails=0 exit=0 out=OK")

	// Fail the check, which should now make it exit
	dockerCmd(c, "exec", "fatal_healthcheck", "rm", "/status")
	waitForStatus(c, "fatal_healthcheck", "running", "exited")

	out, _ = dockerCmd(c, "inspect", "--format={{.State.Health.Status}}", "fatal_healthcheck")
	c.Check(out, checker.Equals, "unhealthy\n")
	failsStr, _ := dockerCmd(c, "inspect", "--format={{.State.Health.FailingStreak}}", "fatal_healthcheck")
	fails, err := strconv.Atoi(strings.TrimSpace(failsStr))
	c.Check(err, check.IsNil)
	c.Check(fails >= 3, checker.Equals, true)
	dockerCmd(c, "rm", "-f", "fatal_healthcheck")

	// Check timeout
	// Note: if the interval is too small, it seems that Docker spends all its time running health
	// checks and never gets around to killing it.
	_, _ = dockerCmd(c, "run", "-d", "--name=test",
		"--health-interval=1s", "--health-cmd=sleep 5m", "--health-timeout=1ms", imageName)
	waitForHealthStatus(c, "test", "starting", "unhealthy")
	out, _ = dockerCmd(c, "inspect",
		"--format=status={{.State.Health.Status}} "+
			"exit={{.State.Health.LastExitCode}} "+
			"out={{.State.Health.LastOutput}}", "test")
	c.Check(strings.TrimSpace(out), checker.Equals, "status=unhealthy exit=-1 out=Health check exceeded timeout (1ms)")
	dockerCmd(c, "rm", "-f", "test")
}
