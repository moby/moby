package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/pkg/timeutils"
	"github.com/go-check/check"
)

// This used to work, it test a log of PageSize-1 (gh#4851)
func (s *DockerSuite) TestLogsContainerSmallerThanPage(c *check.C) {
	testLen := 32767
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo -n =; done; echo", testLen))
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		c.Fatalf("failed to log container: %s, %v", out, err)
	}

	if len(out) != testLen+1 {
		c.Fatalf("Expected log length of %d, received %d\n", testLen+1, len(out))
	}

	deleteContainer(cleanedContainerID)

}

// Regression test: When going over the PageSize, it used to panic (gh#4851)
func (s *DockerSuite) TestLogsContainerBiggerThanPage(c *check.C) {
	testLen := 32768
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo -n =; done; echo", testLen))
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		c.Fatalf("failed to log container: %s, %v", out, err)
	}

	if len(out) != testLen+1 {
		c.Fatalf("Expected log length of %d, received %d\n", testLen+1, len(out))
	}

	deleteContainer(cleanedContainerID)

}

// Regression test: When going much over the PageSize, it used to block (gh#4851)
func (s *DockerSuite) TestLogsContainerMuchBiggerThanPage(c *check.C) {
	testLen := 33000
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo -n =; done; echo", testLen))
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		c.Fatalf("failed to log container: %s, %v", out, err)
	}

	if len(out) != testLen+1 {
		c.Fatalf("Expected log length of %d, received %d\n", testLen+1, len(out))
	}

	deleteContainer(cleanedContainerID)

}

func (s *DockerSuite) TestLogsTimestamps(c *check.C) {
	testLen := 100
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo =; done;", testLen))

	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", "-t", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		c.Fatalf("failed to log container: %s, %v", out, err)
	}

	lines := strings.Split(out, "\n")

	if len(lines) != testLen+1 {
		c.Fatalf("Expected log %d lines, received %d\n", testLen+1, len(lines))
	}

	ts := regexp.MustCompile(`^.* `)

	for _, l := range lines {
		if l != "" {
			_, err := time.Parse(timeutils.RFC3339NanoFixed+" ", ts.FindString(l))
			if err != nil {
				c.Fatalf("Failed to parse timestamp from %v: %v", l, err)
			}
			if l[29] != 'Z' { // ensure we have padded 0's
				c.Fatalf("Timestamp isn't padded properly: %s", l)
			}
		}
	}

	deleteContainer(cleanedContainerID)

}

func (s *DockerSuite) TestLogsSeparateStderr(c *check.C) {
	msg := "stderr_log"
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("echo %s 1>&2", msg))

	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	stdout, stderr, _, err := runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		c.Fatalf("failed to log container: %s, %v", out, err)
	}

	if stdout != "" {
		c.Fatalf("Expected empty stdout stream, got %v", stdout)
	}

	stderr = strings.TrimSpace(stderr)
	if stderr != msg {
		c.Fatalf("Expected %v in stderr stream, got %v", msg, stderr)
	}

	deleteContainer(cleanedContainerID)

}

func (s *DockerSuite) TestLogsStderrInStdout(c *check.C) {
	msg := "stderr_log"
	runCmd := exec.Command(dockerBinary, "run", "-d", "-t", "busybox", "sh", "-c", fmt.Sprintf("echo %s 1>&2", msg))

	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	stdout, stderr, _, err := runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		c.Fatalf("failed to log container: %s, %v", out, err)
	}

	if stderr != "" {
		c.Fatalf("Expected empty stderr stream, got %v", stdout)
	}

	stdout = strings.TrimSpace(stdout)
	if stdout != msg {
		c.Fatalf("Expected %v in stdout stream, got %v", msg, stdout)
	}

	deleteContainer(cleanedContainerID)

}

func (s *DockerSuite) TestLogsTail(c *check.C) {
	testLen := 100
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo =; done;", testLen))

	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", "--tail", "5", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		c.Fatalf("failed to log container: %s, %v", out, err)
	}

	lines := strings.Split(out, "\n")

	if len(lines) != 6 {
		c.Fatalf("Expected log %d lines, received %d\n", 6, len(lines))
	}

	logsCmd = exec.Command(dockerBinary, "logs", "--tail", "all", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		c.Fatalf("failed to log container: %s, %v", out, err)
	}

	lines = strings.Split(out, "\n")

	if len(lines) != testLen+1 {
		c.Fatalf("Expected log %d lines, received %d\n", testLen+1, len(lines))
	}

	logsCmd = exec.Command(dockerBinary, "logs", "--tail", "random", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		c.Fatalf("failed to log container: %s, %v", out, err)
	}

	lines = strings.Split(out, "\n")

	if len(lines) != testLen+1 {
		c.Fatalf("Expected log %d lines, received %d\n", testLen+1, len(lines))
	}

	deleteContainer(cleanedContainerID)
}

func (s *DockerSuite) TestLogsFollowStopped(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "echo", "hello")

	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", "-f", cleanedContainerID)
	if err := logsCmd.Start(); err != nil {
		c.Fatal(err)
	}

	ch := make(chan struct{})
	go func() {
		if err := logsCmd.Wait(); err != nil {
			c.Fatal(err)
		}
		close(ch)
	}()

	select {
	case <-ch:
	case <-time.After(1 * time.Second):
		c.Fatal("Following logs is hanged")
	}

	deleteContainer(cleanedContainerID)
}

// Regression test for #8832
func (s *DockerSuite) TestLogsFollowSlowStdoutConsumer(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "/bin/sh", "-c", `usleep 200000;yes X | head -c 200000`)

	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)
	defer deleteContainer(cleanedContainerID)

	stopSlowRead := make(chan bool)

	go func() {
		exec.Command(dockerBinary, "wait", cleanedContainerID).Run()
		stopSlowRead <- true
	}()

	logCmd := exec.Command(dockerBinary, "logs", "-f", cleanedContainerID)

	stdout, err := logCmd.StdoutPipe()
	if err != nil {
		c.Fatal(err)
	}

	if err := logCmd.Start(); err != nil {
		c.Fatal(err)
	}

	// First read slowly
	bytes1, err := consumeWithSpeed(stdout, 10, 50*time.Millisecond, stopSlowRead)
	if err != nil {
		c.Fatal(err)
	}

	// After the container has finished we can continue reading fast
	bytes2, err := consumeWithSpeed(stdout, 32*1024, 0, nil)
	if err != nil {
		c.Fatal(err)
	}

	actual := bytes1 + bytes2
	expected := 200000
	if actual != expected {
		c.Fatalf("Invalid bytes read: %d, expected %d", actual, expected)
	}

}
