// +build !windows

package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestLinksEtcHostsRegularFile(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "--net=host", "busybox", "ls", "-la", "/etc/hosts")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	if !strings.HasPrefix(out, "-") {
		c.Errorf("/etc/hosts should be a regular file")
	}
}

func (s *DockerSuite) TestLinksEtcHostsContentMatch(c *check.C) {
	testRequires(c, SameHostDaemon)

	runCmd := exec.Command(dockerBinary, "run", "--net=host", "busybox", "cat", "/etc/hosts")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	hosts, err := ioutil.ReadFile("/etc/hosts")
	if os.IsNotExist(err) {
		c.Skip("/etc/hosts does not exist, skip this test")
	}

	if out != string(hosts) {
		c.Errorf("container")
	}

}

func (s *DockerSuite) TestLinksNetworkHostContainer(c *check.C) {

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--net", "host", "--name", "host_container", "busybox", "top"))
	if err != nil {
		c.Fatal(err, out)
	}

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "run", "--name", "should_fail", "--link", "host_container:tester", "busybox", "true"))
	if err == nil || !strings.Contains(out, "--net=host can't be used with links. This would result in undefined behavior") {
		c.Fatalf("Running container linking to a container with --net host should have failed: %s", out)
	}

}
