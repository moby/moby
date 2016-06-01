package main

import (
	"bufio"
	"compress/gzip"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestImportDisplay(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	cleanedContainerID := strings.TrimSpace(out)

	out, _, err := runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "export", cleanedContainerID),
		exec.Command(dockerBinary, "import", "-"),
	)
	c.Assert(err, checker.IsNil)

	c.Assert(out, checker.Count, "\n", 1, check.Commentf("display is expected 1 '\\n' but didn't"))

	image := strings.TrimSpace(out)
	out, _ = dockerCmd(c, "run", "--rm", image, "true")
	c.Assert(out, checker.Equals, "", check.Commentf("command output should've been nothing."))
}

func (s *DockerSuite) TestImportBadURL(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("import", "http://nourl/bad")
	c.Assert(err, checker.NotNil, check.Commentf("import was supposed to fail but didn't"))
	// Depending on your system you can get either of these errors
	if !strings.Contains(out, "dial tcp") &&
		!strings.Contains(out, "Error processing tar file") {
		c.Fatalf("expected an error msg but didn't get one.\nErr: %v\nOut: %v", err, out)
	}
}

func (s *DockerSuite) TestImportFile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name", "test-import", "busybox", "true")

	temporaryFile, err := ioutil.TempFile("", "exportImportTest")
	c.Assert(err, checker.IsNil, check.Commentf("failed to create temporary file"))
	defer os.Remove(temporaryFile.Name())

	runCmd := exec.Command(dockerBinary, "export", "test-import")
	runCmd.Stdout = bufio.NewWriter(temporaryFile)

	_, err = runCommand(runCmd)
	c.Assert(err, checker.IsNil, check.Commentf("failed to export a container"))

	out, _ := dockerCmd(c, "import", temporaryFile.Name())
	c.Assert(out, checker.Count, "\n", 1, check.Commentf("display is expected 1 '\\n' but didn't"))
	image := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "run", "--rm", image, "true")
	c.Assert(out, checker.Equals, "", check.Commentf("command output should've been nothing."))
}

func (s *DockerSuite) TestImportGzipped(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name", "test-import", "busybox", "true")

	temporaryFile, err := ioutil.TempFile("", "exportImportTest")
	c.Assert(err, checker.IsNil, check.Commentf("failed to create temporary file"))
	defer os.Remove(temporaryFile.Name())

	runCmd := exec.Command(dockerBinary, "export", "test-import")
	w := gzip.NewWriter(temporaryFile)
	runCmd.Stdout = w

	_, err = runCommand(runCmd)
	c.Assert(err, checker.IsNil, check.Commentf("failed to export a container"))
	err = w.Close()
	c.Assert(err, checker.IsNil, check.Commentf("failed to close gzip writer"))
	temporaryFile.Close()
	out, _ := dockerCmd(c, "import", temporaryFile.Name())
	c.Assert(out, checker.Count, "\n", 1, check.Commentf("display is expected 1 '\\n' but didn't"))
	image := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "run", "--rm", image, "true")
	c.Assert(out, checker.Equals, "", check.Commentf("command output should've been nothing."))
}

func (s *DockerSuite) TestImportFileWithMessage(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name", "test-import", "busybox", "true")

	temporaryFile, err := ioutil.TempFile("", "exportImportTest")
	c.Assert(err, checker.IsNil, check.Commentf("failed to create temporary file"))
	defer os.Remove(temporaryFile.Name())

	runCmd := exec.Command(dockerBinary, "export", "test-import")
	runCmd.Stdout = bufio.NewWriter(temporaryFile)

	_, err = runCommand(runCmd)
	c.Assert(err, checker.IsNil, check.Commentf("failed to export a container"))

	message := "Testing commit message"
	out, _ := dockerCmd(c, "import", "-m", message, temporaryFile.Name())
	c.Assert(out, checker.Count, "\n", 1, check.Commentf("display is expected 1 '\\n' but didn't"))
	image := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "history", image)
	split := strings.Split(out, "\n")

	c.Assert(split, checker.HasLen, 3, check.Commentf("expected 3 lines from image history"))
	r := regexp.MustCompile("[\\s]{2,}")
	split = r.Split(split[1], -1)

	c.Assert(message, checker.Equals, split[3], check.Commentf("didn't get expected value in commit message"))

	out, _ = dockerCmd(c, "run", "--rm", image, "true")
	c.Assert(out, checker.Equals, "", check.Commentf("command output should've been nothing"))
}

func (s *DockerSuite) TestImportFileNonExistentFile(c *check.C) {
	_, _, err := dockerCmdWithError("import", "example.com/myImage.tar")
	c.Assert(err, checker.NotNil, check.Commentf("import non-existing file must failed"))
}
