package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestInspectImage(c *check.C) {
	imageTest := "emptyfs"
	imageTestID := "511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158"
	imagesCmd := exec.Command(dockerBinary, "inspect", "--format='{{.Id}}'", imageTest)
	out, exitCode, err := runCommandWithOutput(imagesCmd)
	if exitCode != 0 || err != nil {
		c.Fatalf("failed to inspect image: %s, %v", out, err)
	}

	if id := strings.TrimSuffix(out, "\n"); id != imageTestID {
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

	inspectCmd := exec.Command(dockerBinary, "inspect", "-f", "{{.HostConfig.Memory}}", out)
	inspectOut, _, err := runCommandWithOutput(inspectCmd)
	if err != nil {
		c.Fatalf("failed to inspect container: %v, output: %q", err, inspectOut)
	}

	if strings.TrimSpace(inspectOut) != "314572800" {
		c.Fatalf("inspect got wrong value, got: %q, expected: 314572800", inspectOut)
	}
}
