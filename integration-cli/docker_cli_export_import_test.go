package main

import (
	"os"
	"strings"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
	"github.com/gotestyourself/gotestyourself/icmd"
)

// TODO: Move this test to docker/cli, as it is essentially the same test
// as TestExportContainerAndImportImage except output to a file.
// Used to test output flag in the export command
func (s *DockerSuite) TestExportContainerWithOutputAndImportImage(c *check.C) {
	testRequires(c, DaemonIsLinux)
	containerID := "testexportcontainerwithoutputandimportimage"

	dockerCmd(c, "run", "--name", containerID, "busybox", "true")
	dockerCmd(c, "export", "--output=testexp.tar", containerID)
	defer os.Remove("testexp.tar")

	resultCat := icmd.RunCommand("cat", "testexp.tar")
	resultCat.Assert(c, icmd.Success)

	result := icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "import", "-", "repo/testexp:v1"},
		Stdin:   strings.NewReader(resultCat.Combined()),
	})
	result.Assert(c, icmd.Success)

	cleanedImageID := strings.TrimSpace(result.Combined())
	c.Assert(cleanedImageID, checker.Not(checker.Equals), "", check.Commentf("output should have been an image id"))
}
