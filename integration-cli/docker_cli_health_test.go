package main

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"gotest.tools/v3/assert"
)

type DockerCLIHealthSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIHealthSuite) TearDownTest(ctx context.Context, c *testing.T) {
	s.ds.TearDownTest(ctx, c)
}

func (s *DockerCLIHealthSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func waitForHealthStatus(c *testing.T, name string, prev string, expected string) {
	prev = prev + "\n"
	expected = expected + "\n"
	for {
		out := cli.DockerCmd(c, "inspect", "--format={{.State.Health.Status}}", name).Stdout()
		if out == expected {
			return
		}
		assert.Equal(c, out, prev)
		if out != prev {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func getHealth(c *testing.T, name string) *types.Health {
	out := cli.DockerCmd(c, "inspect", "--format={{json .State.Health}}", name).Stdout()
	var health types.Health
	err := json.Unmarshal([]byte(out), &health)
	assert.Equal(c, err, nil)
	return &health
}

func (s *DockerCLIHealthSuite) TestHealth(c *testing.T) {
	testRequires(c, DaemonIsLinux) // busybox doesn't work on Windows

	existingContainers := ExistingContainerIDs(c)

	imageName := "testhealth"
	buildImageSuccessfully(c, imageName, build.WithDockerfile(`FROM busybox
		RUN echo OK > /status
		CMD ["/bin/sleep", "120"]
		STOPSIGNAL SIGKILL
		HEALTHCHECK --interval=1s --timeout=30s \
		  CMD cat /status`))

	// No health status before starting
	name := "test_health"
	cid := cli.DockerCmd(c, "create", "--name", name, imageName).Stdout()
	out := cli.DockerCmd(c, "ps", "-a", "--format={{.ID}} {{.Status}}").Stdout()
	out = RemoveOutputForExistingElements(out, existingContainers)
	assert.Equal(c, out, cid[:12]+" Created\n")

	// Inspect the options
	out = cli.DockerCmd(c, "inspect", "--format=timeout={{.Config.Healthcheck.Timeout}} interval={{.Config.Healthcheck.Interval}} retries={{.Config.Healthcheck.Retries}} test={{.Config.Healthcheck.Test}}", name).Stdout()
	assert.Equal(c, out, "timeout=30s interval=1s retries=0 test=[CMD-SHELL cat /status]\n")

	// Start
	cli.DockerCmd(c, "start", name)
	waitForHealthStatus(c, name, "starting", "healthy")

	// Make it fail
	cli.DockerCmd(c, "exec", name, "rm", "/status")
	waitForHealthStatus(c, name, "healthy", "unhealthy")

	// Inspect the status
	out = cli.DockerCmd(c, "inspect", "--format={{.State.Health.Status}}", name).Stdout()
	assert.Equal(c, out, "unhealthy\n")

	// Make it healthy again
	cli.DockerCmd(c, "exec", name, "touch", "/status")
	waitForHealthStatus(c, name, "unhealthy", "healthy")

	// Remove container
	cli.DockerCmd(c, "rm", "-f", name)

	// Disable the check from the CLI
	cli.DockerCmd(c, "create", "--name=noh", "--no-healthcheck", imageName)
	out = cli.DockerCmd(c, "inspect", "--format={{.Config.Healthcheck.Test}}", "noh").Stdout()
	assert.Equal(c, out, "[NONE]\n")
	cli.DockerCmd(c, "rm", "noh")

	// Disable the check with a new build
	buildImageSuccessfully(c, "no_healthcheck", build.WithDockerfile(`FROM testhealth
		HEALTHCHECK NONE`))

	out = cli.DockerCmd(c, "inspect", "--format={{.Config.Healthcheck.Test}}", "no_healthcheck").Stdout()
	assert.Equal(c, out, "[NONE]\n")

	// Enable the checks from the CLI
	cli.DockerCmd(c, "run", "-d", "--name=fatal_healthcheck",
		"--health-interval=1s",
		"--health-retries=3",
		"--health-cmd=cat /status",
		"no_healthcheck",
	)
	waitForHealthStatus(c, "fatal_healthcheck", "starting", "healthy")
	health := getHealth(c, "fatal_healthcheck")
	assert.Equal(c, health.Status, "healthy")
	assert.Equal(c, health.FailingStreak, 0)
	last := health.Log[len(health.Log)-1]
	assert.Equal(c, last.ExitCode, 0)
	assert.Equal(c, last.Output, "OK\n")

	// Fail the check
	cli.DockerCmd(c, "exec", "fatal_healthcheck", "rm", "/status")
	waitForHealthStatus(c, "fatal_healthcheck", "healthy", "unhealthy")

	failsStr := cli.DockerCmd(c, "inspect", "--format={{.State.Health.FailingStreak}}", "fatal_healthcheck").Combined()
	fails, err := strconv.Atoi(strings.TrimSpace(failsStr))
	assert.Check(c, err)
	assert.Equal(c, fails >= 3, true)
	cli.DockerCmd(c, "rm", "-f", "fatal_healthcheck")

	// Check timeout
	// Note: if the interval is too small, it seems that Docker spends all its time running health
	// checks and never gets around to killing it.
	cli.DockerCmd(c, "run", "-d", "--name=test", "--health-interval=1s", "--health-cmd=sleep 5m", "--health-timeout=1s", imageName)
	waitForHealthStatus(c, "test", "starting", "unhealthy")
	health = getHealth(c, "test")
	last = health.Log[len(health.Log)-1]
	assert.Equal(c, health.Status, "unhealthy")
	assert.Equal(c, last.ExitCode, -1)
	assert.Equal(c, last.Output, "Health check exceeded timeout (1s)")
	cli.DockerCmd(c, "rm", "-f", "test")

	// Check JSON-format
	buildImageSuccessfully(c, imageName, build.WithDockerfile(`FROM busybox
		RUN echo OK > /status
		CMD ["/bin/sleep", "120"]
		STOPSIGNAL SIGKILL
		HEALTHCHECK --interval=1s --timeout=30s \
		  CMD ["cat", "/my status"]`))
	out = cli.DockerCmd(c, "inspect", "--format={{.Config.Healthcheck.Test}}", imageName).Stdout()
	assert.Equal(c, out, "[CMD cat /my status]\n")
}

// GitHub #33021
func (s *DockerCLIHealthSuite) TestUnsetEnvVarHealthCheck(c *testing.T) {
	testRequires(c, DaemonIsLinux) // busybox doesn't work on Windows

	imageName := "testhealth"
	buildImageSuccessfully(c, imageName, build.WithDockerfile(`FROM busybox
HEALTHCHECK --interval=1s --timeout=5s --retries=5 CMD /bin/sh -c "sleep 1"
ENTRYPOINT /bin/sh -c "sleep 600"`))

	name := "env_test_health"
	// No health status before starting
	cli.DockerCmd(c, "run", "-d", "--name", name, "-e", "FOO", imageName)
	defer func() {
		cli.DockerCmd(c, "rm", "-f", name)
		cli.DockerCmd(c, "rmi", imageName)
	}()

	// Start
	cli.DockerCmd(c, "start", name)
	waitForHealthStatus(c, name, "starting", "healthy")
}
