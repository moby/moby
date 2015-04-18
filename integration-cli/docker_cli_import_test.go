package main

import (
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
	defer deleteContainer(cleanedContainerID)

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
	defer deleteImages(image)

	runCmd = exec.Command(dockerBinary, "run", "--rm", image, "true")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal("failed to create a container", out, err)
	}

	if out != "" {
		c.Fatalf("command output should've been nothing, was %q", out)
	}

}
