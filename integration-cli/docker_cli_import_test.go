package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/moby/moby/v2/integration-cli/cli"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
)

type DockerCLIImportSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIImportSuite) TearDownTest(ctx context.Context, t *testing.T) {
	s.ds.TearDownTest(ctx, t)
}

func (s *DockerCLIImportSuite) OnTimeout(t *testing.T) {
	s.ds.OnTimeout(t)
}

func (s *DockerCLIImportSuite) TestImportDisplay(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	cID := cli.DockerCmd(c, "run", "-d", "busybox", "true").Stdout()
	cID = strings.TrimSpace(cID)

	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "export", cID),
		exec.Command(dockerBinary, "import", "-"),
	)
	assert.NilError(c, err)

	assert.Assert(c, strings.Count(out, "\n") == 1, "display is expected 1 '\\n' but didn't")

	imgRef := strings.TrimSpace(out)
	out = cli.DockerCmd(c, "run", "--rm", imgRef, "true").Combined()
	assert.Equal(c, out, "", "command output should've been nothing.")
}

func (s *DockerCLIImportSuite) TestImportBadURL(c *testing.T) {
	out, _, err := dockerCmdWithError("import", "https://nosuchdomain.invalid/bad")
	assert.Assert(c, err != nil, "import was supposed to fail but didn't")
	// Depending on your system you can get either of these errors
	if !strings.Contains(out, "dial tcp") &&
		!strings.Contains(out, "ApplyLayer exit status 1 stdout:  stderr: archive/tar: invalid tar header") &&
		!strings.Contains(out, "Error processing tar file") {
		c.Fatalf("expected an error msg but didn't get one.\nErr: %v\nOut: %v", err, out)
	}
}

func (s *DockerCLIImportSuite) TestImportFile(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	cli.DockerCmd(c, "run", "--name", "test-import", "busybox", "true")

	tmpFile, err := os.CreateTemp(c.TempDir(), "exportImportTest")
	assert.NilError(c, err, "failed to create temporary file")
	c.Cleanup(func() { _ = tmpFile.Close() })

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "export", "test-import"},
		Stdout:  tmpFile,
	}).Assert(c, icmd.Success)

	out := cli.DockerCmd(c, "import", tmpFile.Name()).Combined()
	assert.Assert(c, strings.Count(out, "\n") == 1, "display is expected 1 '\\n' but didn't")
	imgRef := strings.TrimSpace(out)

	out = cli.DockerCmd(c, "run", "--rm", imgRef, "true").Combined()
	assert.Equal(c, out, "", "command output should've been nothing.")
}

func (s *DockerCLIImportSuite) TestImportGzipped(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	cli.DockerCmd(c, "run", "--name", "test-import", "busybox", "true")

	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "export", "test-import"},
		Stdout:  w,
	}).Assert(c, icmd.Success)
	assert.NilError(c, w.Close(), "failed to close gzip writer")

	result := icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "import", "-"},
		Stdin:   &buf,
	}).Assert(c, icmd.Success)

	out := result.Stdout()
	assert.Assert(c, strings.Count(out, "\n") == 1, "display is expected 1 '\\n' but didn't")
	imgRef := strings.TrimSpace(out)

	out = cli.DockerCmd(c, "run", "--rm", imgRef, "true").Combined()
	assert.Equal(c, out, "", "command output should've been nothing.")
}

func (s *DockerCLIImportSuite) TestImportWithMessage(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	cli.DockerCmd(c, "run", "--name", "test-import", "busybox", "true")

	var buf bytes.Buffer
	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "export", "test-import"},
		Stdout:  &buf,
	}).Assert(c, icmd.Success)

	message := "Testing commit message"
	result := icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "import", "-m", message, "-"},
		Stdin:   &buf,
	}).Assert(c, icmd.Success)

	out := result.Stdout()
	assert.Assert(c, strings.Count(out, "\n") == 1, "display is expected 1 '\\n' but didn't")
	imgRef := strings.TrimSpace(out)

	out = cli.DockerCmd(c, "history", imgRef).Combined()
	split := strings.Split(out, "\n")

	assert.Equal(c, len(split), 3, "expected 3 lines from image history")
	r := regexp.MustCompile(`[\s]{2,}`)
	split = r.Split(split[1], -1)

	assert.Equal(c, message, split[3], "didn't get expected value in commit message")

	out = cli.DockerCmd(c, "run", "--rm", imgRef, "true").Combined()
	assert.Equal(c, out, "", "command output should've been nothing")
}

func (s *DockerCLIImportSuite) TestImportFileNonExistentFile(c *testing.T) {
	_, _, err := dockerCmdWithError("import", "example.com/myImage.tar")
	assert.Assert(c, err != nil, "import non-existing file must failed")
}

func (s *DockerCLIImportSuite) TestImportWithQuotedChanges(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	cli.DockerCmd(c, "run", "--name", "test-import", "busybox", "true")

	var buf bytes.Buffer
	cli.Docker(
		cli.Args("export", "test-import"),
		cli.WithStdout(&buf),
	).Assert(c, icmd.Success)

	result := cli.Docker(
		cli.Args("import", "-c", `ENTRYPOINT ["/bin/sh", "-c"]`, "-"),
		cli.WithStdin(&buf),
	)
	result.Assert(c, icmd.Success)

	imgRef := strings.TrimSpace(result.Stdout())

	result = cli.DockerCmd(c, "run", "--rm", imgRef, "true")
	result.Assert(c, icmd.Expected{Out: icmd.None})
}
