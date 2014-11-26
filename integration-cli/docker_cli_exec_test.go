package main

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestExec(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "--name", "testing", "busybox", "sh", "-c", "echo test > /tmp/file && sleep 100")
	if out, _, _, err := runCommandWithStdoutStderr(runCmd); err != nil {
		t.Fatal(out, err)
	}

	execCmd := exec.Command(dockerBinary, "exec", "testing", "cat", "/tmp/file")
	out, _, err := runCommandWithOutput(execCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	out = strings.Trim(out, "\r\n")

	if expected := "test"; out != expected {
		t.Errorf("container exec should've printed %q but printed %q", expected, out)
	}

	deleteAllContainers()

	logDone("exec - basic test")
}

func TestExecInteractiveStdinClose(t *testing.T) {
	defer deleteAllContainers()
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-itd", "busybox", "/bin/cat"))
	if err != nil {
		t.Fatal(err)
	}

	contId := strings.TrimSpace(out)

	returnchan := make(chan struct{})

	go func() {
		var err error
		cmd := exec.Command(dockerBinary, "exec", "-i", contId, "/bin/ls", "/")
		cmd.Stdin = os.Stdin
		if err != nil {
			t.Fatal(err)
		}

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(err, out)
		}

		if string(out) == "" {
			t.Fatalf("Output was empty, likely blocked by standard input")
		}

		returnchan <- struct{}{}
	}()

	select {
	case <-returnchan:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out running docker exec")
	}

	logDone("exec - interactive mode closes stdin after execution")
}

func TestExecInteractive(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "--name", "testing", "busybox", "sh", "-c", "echo test > /tmp/file && sleep 100")
	if out, _, _, err := runCommandWithStdoutStderr(runCmd); err != nil {
		t.Fatal(out, err)
	}

	execCmd := exec.Command(dockerBinary, "exec", "-i", "testing", "sh")
	stdin, err := execCmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := execCmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := execCmd.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := stdin.Write([]byte("cat /tmp/file\n")); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(stdout)
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	line = strings.TrimSpace(line)
	if line != "test" {
		t.Fatalf("Output should be 'test', got '%q'", line)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	finish := make(chan struct{})
	go func() {
		if err := execCmd.Wait(); err != nil {
			t.Fatal(err)
		}
		close(finish)
	}()
	select {
	case <-finish:
	case <-time.After(1 * time.Second):
		t.Fatal("docker exec failed to exit on stdin close")
	}

	deleteAllContainers()

	logDone("exec - Interactive test")
}

func TestExecAfterContainerRestart(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "restart", cleanedContainerID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "exec", cleanedContainerID, "echo", "hello")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	outStr := strings.TrimSpace(out)
	if outStr != "hello" {
		t.Errorf("container should've printed hello, instead printed %q", outStr)
	}

	deleteAllContainers()

	logDone("exec - exec running container after container restart")
}

func TestExecAfterDaemonRestart(t *testing.T) {
	d := NewDaemon(t)
	if err := d.StartWithBusybox(); err != nil {
		t.Fatalf("Could not start daemon with busybox: %v", err)
	}
	defer d.Stop()

	if out, err := d.Cmd("run", "-d", "--name", "top", "-p", "80", "busybox:latest", "top"); err != nil {
		t.Fatalf("Could not run top: err=%v\n%s", err, out)
	}

	if err := d.Restart(); err != nil {
		t.Fatalf("Could not restart daemon: %v", err)
	}

	if out, err := d.Cmd("start", "top"); err != nil {
		t.Fatalf("Could not start top after daemon restart: err=%v\n%s", err, out)
	}

	out, err := d.Cmd("exec", "top", "echo", "hello")
	if err != nil {
		t.Fatalf("Could not exec on container top: err=%v\n%s", err, out)
	}

	outStr := strings.TrimSpace(string(out))
	if outStr != "hello" {
		t.Errorf("container should've printed hello, instead printed %q", outStr)
	}

	logDone("exec - exec running container after daemon restart")
}

// Regresssion test for #9155, #9044
func TestExecEnv(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run",
		"-e", "LALA=value1",
		"-e", "LALA=value2",
		"-d", "--name", "testing", "busybox", "top")
	if out, _, _, err := runCommandWithStdoutStderr(runCmd); err != nil {
		t.Fatal(out, err)
	}

	execCmd := exec.Command(dockerBinary, "exec", "testing", "env")
	out, _, err := runCommandWithOutput(execCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if strings.Contains(out, "LALA=value1") ||
		!strings.Contains(out, "LALA=value2") ||
		!strings.Contains(out, "HOME=/root") {
		t.Errorf("exec env(%q), expect %q, %q", out, "LALA=value2", "HOME=/root")
	}

	logDone("exec - exec inherits correct env")
}

func TestExecExitStatus(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "--name", "top", "busybox", "top")
	if out, _, _, err := runCommandWithStdoutStderr(runCmd); err != nil {
		t.Fatal(out, err)
	}

	// Test normal (non-detached) case first
	cmd := exec.Command(dockerBinary, "exec", "top", "sh", "-c", "exit 23")
	ec, _ := runCommand(cmd)

	if ec != 23 {
		t.Fatalf("Should have had an ExitCode of 23, not: %d", ec)
	}

	logDone("exec - exec non-zero ExitStatus")
}
