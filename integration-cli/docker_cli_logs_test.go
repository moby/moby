package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

// This used to work, it test a log of PageSize-1 (gh#4851)
func TestLogsContainerSmallerThanPage(t *testing.T) {
	testLen := 32767
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo -n =; done; echo", testLen))
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, fmt.Sprintf("run failed with errors: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	errorOut(err, t, fmt.Sprintf("failed to log container: %v %v", out, err))

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
	errorOut(err, t, fmt.Sprintf("run failed with errors: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	errorOut(err, t, fmt.Sprintf("failed to log container: %v %v", out, err))

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
	errorOut(err, t, fmt.Sprintf("run failed with errors: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	errorOut(err, t, fmt.Sprintf("failed to log container: %v %v", out, err))

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
	errorOut(err, t, fmt.Sprintf("run failed with errors: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", "-t", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	errorOut(err, t, fmt.Sprintf("failed to log container: %v %v", out, err))

	lines := strings.Split(out, "\n")

	if len(lines) != testLen+1 {
		t.Fatalf("Expected log %d lines, received %d\n", testLen+1, len(lines))
	}

	ts := regexp.MustCompile(`^.* `)

	for _, l := range lines {
		if l != "" {
			_, err := time.Parse(time.RFC3339Nano+" ", ts.FindString(l))
			if err != nil {
				t.Fatalf("Failed to parse timestamp from %v: %v", l, err)
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
	errorOut(err, t, fmt.Sprintf("run failed with errors: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	stdout, stderr, _, err := runCommandWithStdoutStderr(logsCmd)
	errorOut(err, t, fmt.Sprintf("failed to log container: %v %v", out, err))

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
	errorOut(err, t, fmt.Sprintf("run failed with errors: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	stdout, stderr, _, err := runCommandWithStdoutStderr(logsCmd)
	errorOut(err, t, fmt.Sprintf("failed to log container: %v %v", out, err))

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
	errorOut(err, t, fmt.Sprintf("run failed with errors: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", "--tail", "5", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	errorOut(err, t, fmt.Sprintf("failed to log container: %v %v", out, err))

	lines := strings.Split(out, "\n")

	if len(lines) != 6 {
		t.Fatalf("Expected log %d lines, received %d\n", 6, len(lines))
	}

	logsCmd = exec.Command(dockerBinary, "logs", "--tail", "all", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	errorOut(err, t, fmt.Sprintf("failed to log container: %v %v", out, err))

	lines = strings.Split(out, "\n")

	if len(lines) != testLen+1 {
		t.Fatalf("Expected log %d lines, received %d\n", testLen+1, len(lines))
	}

	logsCmd = exec.Command(dockerBinary, "logs", "--tail", "random", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	errorOut(err, t, fmt.Sprintf("failed to log container: %v %v", out, err))

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
	errorOut(err, t, fmt.Sprintf("run failed with errors: %v", err))

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

func TestLogsFollowRetry(t *testing.T) {
	msg := "follow_retry"
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("echo %s", msg))
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, fmt.Sprintf("run failed with errors: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	// TODO: this should be replaced with waitForContainer
	time.Sleep(500 * time.Millisecond)

	logsCmd := exec.Command(dockerBinary, "logs", "-F", cleanedContainerID)
	stdout, err := logsCmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := logsCmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := logsCmd.Start(); err != nil {
		t.Fatal(err)
	}

	w := make(chan struct{})
	go func() {
		s := bufio.NewScanner(stdout)
		count := 0
		for s.Scan() {
			count++
			if s.Text() != msg {
				t.Fatalf("Log line should be %q, got %q", msg, s.Text())
			}
		}
		if err := s.Err(); err != nil {
			t.Fatal(err)
		}
		if count != 3 {
			res, _ := ioutil.ReadAll(stderr)
			t.Logf("%s", res)
			t.Fatalf("Should be 3 lines in log, got %d", count)
		}
		if err := logsCmd.Wait(); err != nil {
			t.Fatal(err)
		}
		close(w)
	}()

	if err := exec.Command(dockerBinary, "start", cleanedContainerID).Run(); err != nil {
		t.Fatal(err)
	}
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()
	if err := exec.Command(dockerBinary, "start", cleanedContainerID).Run(); err != nil {
		t.Fatal(err)
	}
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()
	if err := exec.Command(dockerBinary, "rm", cleanedContainerID).Run(); err != nil {
		t.Fatal(err)
	}

	select {
	case <-w:
	case <-time.After(5 * time.Second):
		t.Fatal("logs -F didn't exit after container remove")
	}

	logDone("logs - follow-retry")
}
