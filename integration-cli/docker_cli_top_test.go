package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestTopMultipleArgs(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-i", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("failed to start the container: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	topCmd := exec.Command(dockerBinary, "top", cleanedContainerID, "-o", "pid")
	out, _, err = runCommandWithOutput(topCmd)
	if err != nil {
		c.Fatalf("failed to run top: %s, %v", out, err)
	}

	if !strings.Contains(out, "PID") {
		c.Fatalf("did not see PID after top -o pid: %s", out)
	}

}

func (s *DockerSuite) TestTopNonPrivileged(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-i", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("failed to start the container: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	topCmd := exec.Command(dockerBinary, "top", cleanedContainerID)
	out1, _, err := runCommandWithOutput(topCmd)
	if err != nil {
		c.Fatalf("failed to run top: %s, %v", out1, err)
	}

	topCmd = exec.Command(dockerBinary, "top", cleanedContainerID)
	out2, _, err := runCommandWithOutput(topCmd)
	if err != nil {
		c.Fatalf("failed to run top: %s, %v", out2, err)
	}

	killCmd := exec.Command(dockerBinary, "kill", cleanedContainerID)
	if out, _, err = runCommandWithOutput(killCmd); err != nil {
		c.Fatalf("failed to kill container: %s, %v", out, err)
	}

	if !strings.Contains(out1, "top") && !strings.Contains(out2, "top") {
		c.Fatal("top should've listed `top` in the process list, but failed twice")
	} else if !strings.Contains(out1, "top") {
		c.Fatal("top should've listed `top` in the process list, but failed the first time")
	} else if !strings.Contains(out2, "top") {
		c.Fatal("top should've listed `top` in the process list, but failed the second itime")
	}

}

func (s *DockerSuite) TestTopPrivileged(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "--privileged", "-i", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("failed to start the container: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	topCmd := exec.Command(dockerBinary, "top", cleanedContainerID)
	out1, _, err := runCommandWithOutput(topCmd)
	if err != nil {
		c.Fatalf("failed to run top: %s, %v", out1, err)
	}

	topCmd = exec.Command(dockerBinary, "top", cleanedContainerID)
	out2, _, err := runCommandWithOutput(topCmd)
	if err != nil {
		c.Fatalf("failed to run top: %s, %v", out2, err)
	}

	killCmd := exec.Command(dockerBinary, "kill", cleanedContainerID)
	if out, _, err = runCommandWithOutput(killCmd); err != nil {
		c.Fatalf("failed to kill container: %s, %v", out, err)
	}

	if !strings.Contains(out1, "top") && !strings.Contains(out2, "top") {
		c.Fatal("top should've listed `top` in the process list, but failed twice")
	} else if !strings.Contains(out1, "top") {
		c.Fatal("top should've listed `top` in the process list, but failed the first time")
	} else if !strings.Contains(out2, "top") {
		c.Fatal("top should've listed `top` in the process list, but failed the second itime")
	}

}
