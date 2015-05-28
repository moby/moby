package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestInspectImage(c *check.C) {
	imageTest := "emptyfs"
	imageTestID := "511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158"
	id, err := inspectField(imageTest, "Id")
	c.Assert(err, check.IsNil)

	if id != imageTestID {
		c.Fatalf("Expected id: %s for image: %s but received id: %s", imageTestID, imageTest, id)
	}

}

func (s *DockerSuite) TestInspectInt64(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "-m=300M", "busybox", "true")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	out = strings.TrimSpace(out)

	inspectOut, err := inspectField(out, "HostConfig.Memory")
	c.Assert(err, check.IsNil)

	if inspectOut != "314572800" {
		c.Fatalf("inspect got wrong value, got: %q, expected: 314572800", inspectOut)
	}
}

func (s *DockerSuite) TestInspectImageFilterInt(c *check.C) {
	imageTest := "emptyfs"
	out, err := inspectField(imageTest, "Size")
	c.Assert(err, check.IsNil)

	size, err := strconv.Atoi(out)
	if err != nil {
		c.Fatalf("failed to inspect size of the image: %s, %v", out, err)
	}

	//now see if the size turns out to be the same
	formatStr := fmt.Sprintf("--format='{{eq .Size %d}}'", size)
	imagesCmd := exec.Command(dockerBinary, "inspect", formatStr, imageTest)
	out, exitCode, err := runCommandWithOutput(imagesCmd)
	if exitCode != 0 || err != nil {
		c.Fatalf("failed to inspect image: %s, %v", out, err)
	}
	if result, err := strconv.ParseBool(strings.TrimSuffix(out, "\n")); err != nil || !result {
		c.Fatalf("Expected size: %d for image: %s but received size: %s", size, imageTest, strings.TrimSuffix(out, "\n"))
	}
}

func (s *DockerSuite) TestInspectContainerFilterInt(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-i", "-a", "stdin", "busybox", "cat")
	runCmd.Stdin = strings.NewReader("blahblah")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	id := strings.TrimSpace(out)

	out, err = inspectField(id, "State.ExitCode")
	c.Assert(err, check.IsNil)

	exitCode, err := strconv.Atoi(out)
	if err != nil {
		c.Fatalf("failed to inspect exitcode of the container: %s, %v", out, err)
	}

	//now get the exit code to verify
	formatStr := fmt.Sprintf("--format='{{eq .State.ExitCode %d}}'", exitCode)
	runCmd = exec.Command(dockerBinary, "inspect", formatStr, id)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("failed to inspect container: %s, %v", out, err)
	}
	if result, err := strconv.ParseBool(strings.TrimSuffix(out, "\n")); err != nil || !result {
		c.Fatalf("Expected exitcode: %d for container: %s", exitCode, id)
	}
}
