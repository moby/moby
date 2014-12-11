package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/timeutils"
)

// This used to work, it test a log of PageSize-1 (gh#4851)
func TestLogsContainerSmallerThanPage(t *testing.T) {
	testLen := 32767
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo -n =; done; echo", testLen))
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		t.Fatalf("failed to log container: %s, %v", out, err)
	}

	if len(out) != testLen+1 {
		t.Fatalf("Expected log length of %d, received %d\n", testLen+1, len(out))
	}

	deleteContainer(cleanedContainerID)

	logDone("logs - logs container running echo smaller than page size")
}

// Regression test: When going over the PageSize, it used to panic (gh#4851)
func TestLogsContainerBiggerThanPage(t *testing.T) {
	testLen := 32768
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo -n =; done; echo", testLen))
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		t.Fatalf("failed to log container: %s, %v", out, err)
	}

	if len(out) != testLen+1 {
		t.Fatalf("Expected log length of %d, received %d\n", testLen+1, len(out))
	}

	deleteContainer(cleanedContainerID)

	logDone("logs - logs container running echo bigger than page size")
}

// Regression test: When going much over the PageSize, it used to block (gh#4851)
func TestLogsContainerMuchBiggerThanPage(t *testing.T) {
	testLen := 33000
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo -n =; done; echo", testLen))
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		t.Fatalf("failed to log container: %s, %v", out, err)
	}

	if len(out) != testLen+1 {
		t.Fatalf("Expected log length of %d, received %d\n", testLen+1, len(out))
	}

	deleteContainer(cleanedContainerID)

	logDone("logs - logs container running echo much bigger than page size")
}

func TestLogsTimestamps(t *testing.T) {
	testLen := 100
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo =; done;", testLen))

	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", "-t", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		t.Fatalf("failed to log container: %s, %v", out, err)
	}

	lines := strings.Split(out, "\n")

	if len(lines) != testLen+1 {
		t.Fatalf("Expected log %d lines, received %d\n", testLen+1, len(lines))
	}

	ts := regexp.MustCompile(`^.* `)

	for _, l := range lines {
		if l != "" {
			_, err := time.Parse(timeutils.RFC3339NanoFixed+" ", ts.FindString(l))
			if err != nil {
				t.Fatalf("Failed to parse timestamp from %v: %v", l, err)
			}
			if l[29] != 'Z' { // ensure we have padded 0's
				t.Fatalf("Timestamp isn't padded properly: %s", l)
			}
		}
	}

	deleteContainer(cleanedContainerID)

	logDone("logs - logs with timestamps")
}

func TestLogsSeparateStderr(t *testing.T) {
	msg := "stderr_log"
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("echo %s 1>&2", msg))

	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	stdout, stderr, _, err := runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		t.Fatalf("failed to log container: %s, %v", out, err)
	}

	if stdout != "" {
		t.Fatalf("Expected empty stdout stream, got %v", stdout)
	}

	stderr = strings.TrimSpace(stderr)
	if stderr != msg {
		t.Fatalf("Expected %v in stderr stream, got %v", msg, stderr)
	}

	deleteContainer(cleanedContainerID)

	logDone("logs - separate stderr (without pseudo-tty)")
}

func TestLogsStderrInStdout(t *testing.T) {
	msg := "stderr_log"
	runCmd := exec.Command(dockerBinary, "run", "-d", "-t", "busybox", "sh", "-c", fmt.Sprintf("echo %s 1>&2", msg))

	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	stdout, stderr, _, err := runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		t.Fatalf("failed to log container: %s, %v", out, err)
	}

	if stderr != "" {
		t.Fatalf("Expected empty stderr stream, got %v", stdout)
	}

	stdout = strings.TrimSpace(stdout)
	if stdout != msg {
		t.Fatalf("Expected %v in stdout stream, got %v", msg, stdout)
	}

	deleteContainer(cleanedContainerID)

	logDone("logs - stderr in stdout (with pseudo-tty)")
}

func TestLogsTail(t *testing.T) {
	testLen := 100
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo =; done;", testLen))

	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", "--tail", "5", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		t.Fatalf("failed to log container: %s, %v", out, err)
	}

	lines := strings.Split(out, "\n")

	if len(lines) != 6 {
		t.Fatalf("Expected log %d lines, received %d\n", 6, len(lines))
	}

	logsCmd = exec.Command(dockerBinary, "logs", "--tail", "all", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		t.Fatalf("failed to log container: %s, %v", out, err)
	}

	lines = strings.Split(out, "\n")

	if len(lines) != testLen+1 {
		t.Fatalf("Expected log %d lines, received %d\n", testLen+1, len(lines))
	}

	logsCmd = exec.Command(dockerBinary, "logs", "--tail", "random", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	if err != nil {
		t.Fatalf("failed to log container: %s, %v", out, err)
	}

	lines = strings.Split(out, "\n")

	if len(lines) != testLen+1 {
		t.Fatalf("Expected log %d lines, received %d\n", testLen+1, len(lines))
	}

	deleteContainer(cleanedContainerID)
	logDone("logs - logs tail")
}

func TestLogsFollowStopped(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "echo", "hello")

	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", "-f", cleanedContainerID)
	if err := logsCmd.Start(); err != nil {
		t.Fatal(err)
	}

	c := make(chan struct{})
	go func() {
		if err := logsCmd.Wait(); err != nil {
			t.Fatal(err)
		}
		close(c)
	}()

	select {
	case <-c:
	case <-time.After(1 * time.Second):
		t.Fatal("Following logs is hanged")
	}

	deleteContainer(cleanedContainerID)
	logDone("logs - logs follow stopped container")
}

// Regression test for #8832
func TestLogsFollowSlowStdoutConsumer(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "/bin/sh", "-c", `usleep 200000;yes X | head -c 200000`)

	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("run failed with errors: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	defer deleteContainer(cleanedContainerID)

	stopSlowRead := make(chan bool)

	go func() {
		exec.Command(dockerBinary, "wait", cleanedContainerID).Run()
		stopSlowRead <- true
	}()

	logCmd := exec.Command(dockerBinary, "logs", "-f", cleanedContainerID)

	stdout, err := logCmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := logCmd.Start(); err != nil {
		t.Fatal(err)
	}

	// First read slowly
	bytes1, err := consumeWithSpeed(stdout, 10, 50*time.Millisecond, stopSlowRead)
	if err != nil {
		t.Fatal(err)
	}

	// After the container has finished we can continue reading fast
	bytes2, err := consumeWithSpeed(stdout, 32*1024, 0, nil)
	if err != nil {
		t.Fatal(err)
	}

	actual := bytes1 + bytes2
	expected := 200000
	if actual != expected {
		t.Fatalf("Invalid bytes read: %d, expected %d", actual, expected)
	}

	logDone("logs - follow slow consumer")
}
