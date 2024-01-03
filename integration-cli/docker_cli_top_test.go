package main

import (
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/integration-cli/cli"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
)

type DockerCLITopSuite struct {
	ds *DockerSuite
}

func (s *DockerCLITopSuite) TearDownTest(ctx context.Context, c *testing.T) {
	s.ds.TearDownTest(ctx, c)
}

func (s *DockerCLITopSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func (s *DockerCLITopSuite) TestTopMultipleArgs(c *testing.T) {
	out := runSleepingContainer(c, "-d")
	cleanedContainerID := strings.TrimSpace(out)

	var expected icmd.Expected
	switch testEnv.DaemonInfo.OSType {
	case "windows":
		expected = icmd.Expected{ExitCode: 1, Err: "Windows does not support arguments to top"}
	default:
		expected = icmd.Expected{Out: "PID"}
	}
	result := cli.Docker(cli.Args("top", cleanedContainerID, "-o", "pid"))
	result.Assert(c, expected)
}

func (s *DockerCLITopSuite) TestTopNonPrivileged(c *testing.T) {
	out := runSleepingContainer(c, "-d")
	cleanedContainerID := strings.TrimSpace(out)

	out1 := cli.DockerCmd(c, "top", cleanedContainerID).Combined()
	out2 := cli.DockerCmd(c, "top", cleanedContainerID).Combined()
	cli.DockerCmd(c, "kill", cleanedContainerID)

	// Windows will list the name of the launched executable which in this case is busybox.exe, without the parameters.
	// Linux will display the command executed in the container
	var lookingFor string
	if testEnv.DaemonInfo.OSType == "windows" {
		lookingFor = "busybox.exe"
	} else {
		lookingFor = "top"
	}

	assert.Assert(c, strings.Contains(out1, lookingFor), "top should've listed `%s` in the process list, but failed the first time", lookingFor)
	assert.Assert(c, strings.Contains(out2, lookingFor), "top should've listed `%s` in the process list, but failed the second time", lookingFor)
}

// TestTopWindowsCoreProcesses validates that there are lines for the critical
// processes which are found in a Windows container. Note Windows is architecturally
// very different to Linux in this regard.
func (s *DockerCLITopSuite) TestTopWindowsCoreProcesses(c *testing.T) {
	testRequires(c, DaemonIsWindows)
	cID := runSleepingContainer(c, "-d")
	out1 := cli.DockerCmd(c, "top", cID).Combined()
	lookingFor := []string{"smss.exe", "csrss.exe", "wininit.exe", "services.exe", "lsass.exe", "CExecSvc.exe"}
	for i, s := range lookingFor {
		assert.Assert(c, strings.Contains(out1, s), "top should've listed `%s` in the process list, but failed. Test case %d", s, i)
	}
}

func (s *DockerCLITopSuite) TestTopPrivileged(c *testing.T) {
	// Windows does not support --privileged
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	cID := cli.DockerCmd(c, "run", "--privileged", "-i", "-d", "busybox", "top").Stdout()
	cID = strings.TrimSpace(cID)

	out1 := cli.DockerCmd(c, "top", cID).Combined()
	out2 := cli.DockerCmd(c, "top", cID).Combined()
	cli.DockerCmd(c, "kill", cID)

	assert.Assert(c, strings.Contains(out1, "top"), "top should've listed `top` in the process list, but failed the first time")
	assert.Assert(c, strings.Contains(out2, "top"), "top should've listed `top` in the process list, but failed the second time")
}
