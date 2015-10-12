package main

import (
	"os/exec"
	"strings"

	"github.com/docker/docker/runconfig"
	"github.com/go-check/check"
)

// GH14530. Validates combinations of --net= with other options

// stringCheckPS is how the output of PS starts in order to validate that
// the command executed in a container did really run PS correctly.
const stringCheckPS = "PID   USER"

// checkContains is a helper function that validates a command output did
// contain what was expected.
func checkContains(expected string, out string, c *check.C) {
	if !strings.Contains(out, expected) {
		c.Fatalf("Expected '%s', got '%s'", expected, out)
	}
}

func (s *DockerSuite) TestNetHostname(c *check.C) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)

	var (
		out    string
		err    error
		runCmd *exec.Cmd
	)

	runCmd = exec.Command(dockerBinary, "run", "-h=name", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatalf(out, err)
	}
	checkContains(stringCheckPS, out, c)

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatalf(out, err)
	}
	checkContains(stringCheckPS, out, c)

	runCmd = exec.Command(dockerBinary, "run", "-h=name", "--net=bridge", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatalf(out, err)
	}
	checkContains(stringCheckPS, out, c)

	runCmd = exec.Command(dockerBinary, "run", "-h=name", "--net=none", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatalf(out, err)
	}
	checkContains(stringCheckPS, out, c)

	runCmd = exec.Command(dockerBinary, "run", "-h=name", "--net=host", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err)
	}
	checkContains(runconfig.ErrConflictNetworkHostname.Error(), out, c)

	runCmd = exec.Command(dockerBinary, "run", "-h=name", "--net=container:other", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err, c)
	}
	checkContains(runconfig.ErrConflictNetworkHostname.Error(), out, c)

	runCmd = exec.Command(dockerBinary, "run", "--net=container", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err, c)
	}
	checkContains("--net: invalid net mode: invalid container format container:<name|id>", out, c)

	runCmd = exec.Command(dockerBinary, "run", "--net=weird", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err)
	}
	checkContains("network weird not found", out, c)
}

func (s *DockerSuite) TestConflictContainerNetworkAndLinks(c *check.C) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	var (
		out    string
		err    error
		runCmd *exec.Cmd
	)

	runCmd = exec.Command(dockerBinary, "run", "--net=container:other", "--link=zip:zap", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err)
	}
	checkContains(runconfig.ErrConflictContainerNetworkAndLinks.Error(), out, c)

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "--link=zip:zap", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err)
	}
	checkContains(runconfig.ErrConflictHostNetworkAndLinks.Error(), out, c)
}

func (s *DockerSuite) TestConflictNetworkModeAndOptions(c *check.C) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	var (
		out    string
		err    error
		runCmd *exec.Cmd
	)

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "--dns=8.8.8.8", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err)
	}
	checkContains(runconfig.ErrConflictNetworkAndDNS.Error(), out, c)

	runCmd = exec.Command(dockerBinary, "run", "--net=container:other", "--dns=8.8.8.8", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err)
	}
	checkContains(runconfig.ErrConflictNetworkAndDNS.Error(), out, c)

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "--add-host=name:8.8.8.8", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err)
	}
	checkContains(runconfig.ErrConflictNetworkHosts.Error(), out, c)

	runCmd = exec.Command(dockerBinary, "run", "--net=container:other", "--add-host=name:8.8.8.8", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err)
	}
	checkContains(runconfig.ErrConflictNetworkHosts.Error(), out, c)

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "--mac-address=92:d0:c6:0a:29:33", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err)
	}
	checkContains(runconfig.ErrConflictContainerNetworkAndMac.Error(), out, c)

	runCmd = exec.Command(dockerBinary, "run", "--net=container:other", "--mac-address=92:d0:c6:0a:29:33", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err)
	}
	checkContains(runconfig.ErrConflictContainerNetworkAndMac.Error(), out, c)

	runCmd = exec.Command(dockerBinary, "run", "--net=container:other", "-P", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err)
	}
	checkContains(runconfig.ErrConflictNetworkPublishPorts.Error(), out, c)

	runCmd = exec.Command(dockerBinary, "run", "--net=container:other", "-p", "8080", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err)
	}
	checkContains(runconfig.ErrConflictNetworkPublishPorts.Error(), out, c)

	runCmd = exec.Command(dockerBinary, "run", "--net=container:other", "--expose", "8000-9000", "busybox", "ps")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Fatalf(out, err)
	}
	checkContains(runconfig.ErrConflictNetworkExposePorts.Error(), out, c)
}
