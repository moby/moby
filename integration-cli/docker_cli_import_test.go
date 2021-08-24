package main

import (
	"bufio"
	"compress/gzip"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/docker/docker/integration-cli/cli"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
)

func (s *DockerSuite) TestImportDisplay(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	cleanedContainerID := strings.TrimSpace(out)

	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "export", cleanedContainerID),
		exec.Command(dockerBinary, "import", "-"),
	)
	assert.NilError(c, err)

	assert.Assert(c, strings.Count(out, "\n") == 1, "display is expected 1 '\\n' but didn't")

	image := strings.TrimSpace(out)
	out, _ = dockerCmd(c, "run", "--rm", image, "true")
	assert.Equal(c, out, "", "command output should've been nothing.")
}

func (s *DockerSuite) TestImportBadURL(c *testing.T) {
	out, _, err := dockerCmdWithError("import", "http://nourl/bad")
	assert.Assert(c, err != nil, "import was supposed to fail but didn't")
	// Depending on your system you can get either of these errors
	if !strings.Contains(out, "dial tcp") &&
		!strings.Contains(out, "ApplyLayer exit status 1 stdout:  stderr: archive/tar: invalid tar header") &&
		!strings.Contains(out, "Error processing tar file") {
		c.Fatalf("expected an error msg but didn't get one.\nErr: %v\nOut: %v", err, out)
	}
}

func (s *DockerSuite) TestImportFile(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name", "test-import", "busybox", "true")

	temporaryFile, err := os.CreateTemp("", "exportImportTest")
	assert.Assert(c, err == nil, "failed to create temporary file")
	defer os.Remove(temporaryFile.Name())

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "export", "test-import"},
		Stdout:  bufio.NewWriter(temporaryFile),
	}).Assert(c, icmd.Success)

	out, _ := dockerCmd(c, "import", temporaryFile.Name())
	assert.Assert(c, strings.Count(out, "\n") == 1, "display is expected 1 '\\n' but didn't")
	image := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "run", "--rm", image, "true")
	assert.Equal(c, out, "", "command output should've been nothing.")
}

func (s *DockerSuite) TestImportGzipped(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name", "test-import", "busybox", "true")

	temporaryFile, err := os.CreateTemp("", "exportImportTest")
	assert.Assert(c, err == nil, "failed to create temporary file")
	defer os.Remove(temporaryFile.Name())

	w := gzip.NewWriter(temporaryFile)
	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "export", "test-import"},
		Stdout:  w,
	}).Assert(c, icmd.Success)
	assert.Assert(c, w.Close() == nil, "failed to close gzip writer")
	temporaryFile.Close()
	out, _ := dockerCmd(c, "import", temporaryFile.Name())
	assert.Assert(c, strings.Count(out, "\n") == 1, "display is expected 1 '\\n' but didn't")
	image := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "run", "--rm", image, "true")
	assert.Equal(c, out, "", "command output should've been nothing.")
}

func (s *DockerSuite) TestImportFileWithMessage(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name", "test-import", "busybox", "true")

	temporaryFile, err := os.CreateTemp("", "exportImportTest")
	assert.Assert(c, err == nil, "failed to create temporary file")
	defer os.Remove(temporaryFile.Name())

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "export", "test-import"},
		Stdout:  bufio.NewWriter(temporaryFile),
	}).Assert(c, icmd.Success)

	message := "Testing commit message"
	out, _ := dockerCmd(c, "import", "-m", message, temporaryFile.Name())
	assert.Assert(c, strings.Count(out, "\n") == 1, "display is expected 1 '\\n' but didn't")
	image := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "history", image)
	split := strings.Split(out, "\n")

	assert.Equal(c, len(split), 3, "expected 3 lines from image history")
	r := regexp.MustCompile(`[\s]{2,}`)
	split = r.Split(split[1], -1)

	assert.Equal(c, message, split[3], "didn't get expected value in commit message")

	out, _ = dockerCmd(c, "run", "--rm", image, "true")
	assert.Equal(c, out, "", "command output should've been nothing")
}

func (s *DockerSuite) TestImportFileNonExistentFile(c *testing.T) {
	_, _, err := dockerCmdWithError("import", "example.com/myImage.tar")
	assert.Assert(c, err != nil, "import non-existing file must failed")
}

func (s *DockerSuite) TestImportWithQuotedChanges(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	cli.DockerCmd(c, "run", "--name", "test-import", "busybox", "true")

	temporaryFile, err := os.CreateTemp("", "exportImportTest")
	assert.Assert(c, err == nil, "failed to create temporary file")
	defer os.Remove(temporaryFile.Name())

	cli.Docker(cli.Args("export", "test-import"), cli.WithStdout(bufio.NewWriter(temporaryFile))).Assert(c, icmd.Success)

	result := cli.DockerCmd(c, "import", "-c", `ENTRYPOINT ["/bin/sh", "-c"]`, temporaryFile.Name())
	image := strings.TrimSpace(result.Stdout())

	result = cli.DockerCmd(c, "run", "--rm", image, "true")
	result.Assert(c, icmd.Expected{Out: icmd.None})
}
