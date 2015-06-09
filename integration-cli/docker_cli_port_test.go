package main

import (
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestPortList(c *check.C) {

	// one port
	runCmd := exec.Command(dockerBinary, "run", "-d", "-p", "9876:80", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	firstID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "port", firstID, "80")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	if !assertPortList(c, out, []string{"0.0.0.0:9876"}) {
		c.Error("Port list is not correct")
	}

	runCmd = exec.Command(dockerBinary, "port", firstID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	if !assertPortList(c, out, []string{"80/tcp -> 0.0.0.0:9876"}) {
		c.Error("Port list is not correct")
	}
	runCmd = exec.Command(dockerBinary, "rm", "-f", firstID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	// three port
	runCmd = exec.Command(dockerBinary, "run", "-d",
		"-p", "9876:80",
		"-p", "9877:81",
		"-p", "9878:82",
		"busybox", "top")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	ID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "port", ID, "80")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	if !assertPortList(c, out, []string{"0.0.0.0:9876"}) {
		c.Error("Port list is not correct")
	}

	runCmd = exec.Command(dockerBinary, "port", ID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	if !assertPortList(c, out, []string{
		"80/tcp -> 0.0.0.0:9876",
		"81/tcp -> 0.0.0.0:9877",
		"82/tcp -> 0.0.0.0:9878"}) {
		c.Error("Port list is not correct")
	}
	runCmd = exec.Command(dockerBinary, "rm", "-f", ID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	// more and one port mapped to the same container port
	runCmd = exec.Command(dockerBinary, "run", "-d",
		"-p", "9876:80",
		"-p", "9999:80",
		"-p", "9877:81",
		"-p", "9878:82",
		"busybox", "top")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	ID = strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "port", ID, "80")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	if !assertPortList(c, out, []string{"0.0.0.0:9876", "0.0.0.0:9999"}) {
		c.Error("Port list is not correct")
	}

	runCmd = exec.Command(dockerBinary, "port", ID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	if !assertPortList(c, out, []string{
		"80/tcp -> 0.0.0.0:9876",
		"80/tcp -> 0.0.0.0:9999",
		"81/tcp -> 0.0.0.0:9877",
		"82/tcp -> 0.0.0.0:9878"}) {
		c.Error("Port list is not correct\n", out)
	}
	runCmd = exec.Command(dockerBinary, "rm", "-f", ID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

}

func assertPortList(c *check.C, out string, expected []string) bool {
	//lines := strings.Split(out, "\n")
	lines := strings.Split(strings.Trim(out, "\n "), "\n")
	if len(lines) != len(expected) {
		c.Errorf("different size lists %s, %d, %d", out, len(lines), len(expected))
		return false
	}
	sort.Strings(lines)
	sort.Strings(expected)

	for i := 0; i < len(expected); i++ {
		if lines[i] != expected[i] {
			c.Error("|" + lines[i] + "!=" + expected[i] + "|")
			return false
		}
	}

	return true
}

func (s *DockerSuite) TestPortHostBinding(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "-p", "9876:80", "busybox",
		"nc", "-l", "-p", "80")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	firstID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "port", firstID, "80")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	if !assertPortList(c, out, []string{"0.0.0.0:9876"}) {
		c.Error("Port list is not correct")
	}

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "busybox",
		"nc", "localhost", "9876")
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "rm", "-f", firstID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "busybox",
		"nc", "localhost", "9876")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Error("Port is still bound after the Container is removed")
	}
}

func (s *DockerSuite) TestPortExposeHostBinding(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "-P", "--expose", "80", "busybox",
		"nc", "-l", "-p", "80")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	firstID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "port", firstID, "80")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	_, exposedPort, err := net.SplitHostPort(out)

	if err != nil {
		c.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "busybox",
		"nc", "localhost", strings.TrimSpace(exposedPort))
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "rm", "-f", firstID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "busybox",
		"nc", "localhost", strings.TrimSpace(exposedPort))
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Error("Port is still bound after the Container is removed")
	}
}

func stopRemoveContainer(id string, c *check.C) {
	runCmd := exec.Command(dockerBinary, "rm", "-f", id)
	_, _, err := runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)
}

func (s *DockerSuite) TestUnpublishedPortsInPsOutput(c *check.C) {
	// Run busybox with command line expose (equivalent to EXPOSE in image's Dockerfile) for the following ports
	port1 := 80
	port2 := 443
	expose1 := fmt.Sprintf("--expose=%d", port1)
	expose2 := fmt.Sprintf("--expose=%d", port2)
	runCmd := exec.Command(dockerBinary, "run", "-d", expose1, expose2, "busybox", "sleep", "5")
	out, _, err := runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)

	// Check docker ps o/p for last created container reports the unpublished ports
	unpPort1 := fmt.Sprintf("%d/tcp", port1)
	unpPort2 := fmt.Sprintf("%d/tcp", port2)
	runCmd = exec.Command(dockerBinary, "ps", "-n=1")
	out, _, err = runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)
	if !strings.Contains(out, unpPort1) || !strings.Contains(out, unpPort2) {
		c.Errorf("Missing unpublished ports(s) (%s, %s) in docker ps output: %s", unpPort1, unpPort2, out)
	}

	// Run the container forcing to publish the exposed ports
	runCmd = exec.Command(dockerBinary, "run", "-d", "-P", expose1, expose2, "busybox", "sleep", "5")
	out, _, err = runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)

	// Check docker ps o/p for last created container reports the exposed ports in the port bindings
	expBndRegx1 := regexp.MustCompile(`0.0.0.0:\d\d\d\d\d->` + unpPort1)
	expBndRegx2 := regexp.MustCompile(`0.0.0.0:\d\d\d\d\d->` + unpPort2)
	runCmd = exec.Command(dockerBinary, "ps", "-n=1")
	out, _, err = runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)
	if !expBndRegx1.MatchString(out) || !expBndRegx2.MatchString(out) {
		c.Errorf("Cannot find expected port binding ports(s) (0.0.0.0:xxxxx->%s, 0.0.0.0:xxxxx->%s) in docker ps output:\n%s",
			unpPort1, unpPort2, out)
	}

	// Run the container specifying explicit port bindings for the exposed ports
	offset := 10000
	pFlag1 := fmt.Sprintf("%d:%d", offset+port1, port1)
	pFlag2 := fmt.Sprintf("%d:%d", offset+port2, port2)
	runCmd = exec.Command(dockerBinary, "run", "-d", "-p", pFlag1, "-p", pFlag2, expose1, expose2, "busybox", "sleep", "5")
	out, _, err = runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)
	id := strings.TrimSpace(out)

	// Check docker ps o/p for last created container reports the specified port mappings
	expBnd1 := fmt.Sprintf("0.0.0.0:%d->%s", offset+port1, unpPort1)
	expBnd2 := fmt.Sprintf("0.0.0.0:%d->%s", offset+port2, unpPort2)
	runCmd = exec.Command(dockerBinary, "ps", "-n=1")
	out, _, err = runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)
	if !strings.Contains(out, expBnd1) || !strings.Contains(out, expBnd2) {
		c.Errorf("Cannot find expected port binding(s) (%s, %s) in docker ps output: %s", expBnd1, expBnd2, out)
	}
	// Remove container now otherwise it will interfeer with next test
	stopRemoveContainer(id, c)

	// Run the container with explicit port bindings and no exposed ports
	runCmd = exec.Command(dockerBinary, "run", "-d", "-p", pFlag1, "-p", pFlag2, "busybox", "sleep", "5")
	out, _, err = runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)
	id = strings.TrimSpace(out)

	// Check docker ps o/p for last created container reports the specified port mappings
	runCmd = exec.Command(dockerBinary, "ps", "-n=1")
	out, _, err = runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)
	if !strings.Contains(out, expBnd1) || !strings.Contains(out, expBnd2) {
		c.Errorf("Cannot find expected port binding(s) (%s, %s) in docker ps output: %s", expBnd1, expBnd2, out)
	}
	// Remove container now otherwise it will interfeer with next test
	stopRemoveContainer(id, c)

	// Run the container with one unpublished exposed port and one explicit port binding
	runCmd = exec.Command(dockerBinary, "run", "-d", expose1, "-p", pFlag2, "busybox", "sleep", "5")
	out, _, err = runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)

	// Check docker ps o/p for last created container reports the specified unpublished port and port mapping
	runCmd = exec.Command(dockerBinary, "ps", "-n=1")
	out, _, err = runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)
	if !strings.Contains(out, unpPort1) || !strings.Contains(out, expBnd2) {
		c.Errorf("Missing unpublished ports or port binding (%s, %s) in docker ps output: %s", unpPort1, expBnd2, out)
	}
}
