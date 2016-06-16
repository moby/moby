package main

import (
	"os"
	"os/exec"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

// export an image and try to import it into a new one
func (s *DockerSuite) TestExportContainerAndImportImage(c *check.C) {
	testRequires(c, DaemonIsLinux)
	containerID := "testexportcontainerandimportimage"

	dockerCmd(c, "run", "--name", containerID, "busybox", "true")

	out, _ := dockerCmd(c, "export", containerID)

	importCmd := exec.Command(dockerBinary, "import", "-", "repo/testexp:v1")
	importCmd.Stdin = strings.NewReader(out)
	out, _, err := runCommandWithOutput(importCmd)
	c.Assert(err, checker.IsNil, check.Commentf("failed to import image repo/testexp:v1: %s", out))

	cleanedImageID := strings.TrimSpace(out)
	c.Assert(cleanedImageID, checker.Not(checker.Equals), "", check.Commentf("output should have been an image id"))
}

// Used to test output flag in the export command
func (s *DockerSuite) TestExportContainerWithOutputAndImportImage(c *check.C) {
	testRequires(c, DaemonIsLinux)
	containerID := "testexportcontainerwithoutputandimportimage"

	dockerCmd(c, "run", "--name", containerID, "busybox", "true")
	dockerCmd(c, "export", "--output=testexp.tar", containerID)
	defer os.Remove("testexp.tar")

	out, _, err := runCommandWithOutput(exec.Command("cat", "testexp.tar"))
	c.Assert(err, checker.IsNil, check.Commentf(out))

	importCmd := exec.Command(dockerBinary, "import", "-", "repo/testexp:v1")
	importCmd.Stdin = strings.NewReader(out)
	out, _, err = runCommandWithOutput(importCmd)
	c.Assert(err, checker.IsNil, check.Commentf("failed to import image repo/testexp:v1: %s", out))

	cleanedImageID := strings.TrimSpace(out)
	c.Assert(cleanedImageID, checker.Not(checker.Equals), "", check.Commentf("output should have been an image id"))
}
