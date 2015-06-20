package main

import (
	"bufio"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestImportDisplay(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal("failed to create a container", out, err)
	}
	cleanedContainerID := strings.TrimSpace(out)

	out, _, err = runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "export", cleanedContainerID),
		exec.Command(dockerBinary, "import", "-"),
	)
	if err != nil {
		c.Errorf("import failed with errors: %v, output: %q", err, out)
	}

	if n := strings.Count(out, "\n"); n != 1 {
		c.Fatalf("display is messed up: %d '\\n' instead of 1:\n%s", n, out)
	}
	image := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "run", "--rm", image, "true")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal("failed to create a container", out, err)
	}

	if out != "" {
		c.Fatalf("command output should've been nothing, was %q", out)
	}

}

func (s *DockerSuite) TestImportBadURL(c *check.C) {
	runCmd := exec.Command(dockerBinary, "import", "http://nourl/bad")
	out, _, err := runCommandWithOutput(runCmd)
	if err == nil {
		c.Fatal("import was supposed to fail but didn't")
	}
	if !strings.Contains(out, "dial tcp") {
		c.Fatalf("expected an error msg but didn't get one:\n%s", out)
	}
}

func (s *DockerSuite) TestImportFile(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "test-import", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal("failed to create a container", out, err)
	}

	temporaryFile, err := ioutil.TempFile("", "exportImportTest")
	if err != nil {
		c.Fatal("failed to create temporary file", "", err)
	}
	defer os.Remove(temporaryFile.Name())

	runCmd = exec.Command(dockerBinary, "export", "test-import")
	runCmd.Stdout = bufio.NewWriter(temporaryFile)

	_, err = runCommand(runCmd)
	if err != nil {
		c.Fatal("failed to export a container", out, err)
	}

	runCmd = exec.Command(dockerBinary, "import", temporaryFile.Name())
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal("failed to import a container", out, err)
	}

	if n := strings.Count(out, "\n"); n != 1 {
		c.Fatalf("display is messed up: %d '\\n' instead of 1:\n%s", n, out)
	}
	image := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "run", "--rm", image, "true")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal("failed to create a container", out, err)
	}

	if out != "" {
		c.Fatalf("command output should've been nothing, was %q", out)
	}

}

func (s *DockerSuite) TestImportFileNonExistentFile(c *check.C) {
	runCmd := exec.Command(dockerBinary, "import", "example.com/myImage.tar")
	_, exitCode, err := runCommandWithOutput(runCmd)
	if exitCode == 0 || err == nil {
		c.Fatalf("import non-existing file must failed")
	}

}
