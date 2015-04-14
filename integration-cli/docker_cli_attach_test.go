package main

import (
	"bufio"
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

const attachWait = 5 * time.Second

func TestAttachMultipleAndRestart(t *testing.T) {
	defer deleteAllContainers()

	endGroup := &sync.WaitGroup{}
	startGroup := &sync.WaitGroup{}
	endGroup.Add(3)
	startGroup.Add(3)

	if err := waitForContainer("attacher", "-d", "busybox", "/bin/sh", "-c", "while true; do sleep 1; echo hello; done"); err != nil {
		t.Fatal(err)
	}

	startDone := make(chan struct{})
	endDone := make(chan struct{})

	go func() {
		endGroup.Wait()
		close(endDone)
	}()

	go func() {
		startGroup.Wait()
		close(startDone)
	}()

	for i := 0; i < 3; i++ {
		go func() {
			c := exec.Command(dockerBinary, "attach", "attacher")

			defer func() {
				c.Wait()
				endGroup.Done()
			}()

			out, err := c.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}

			if err := c.Start(); err != nil {
				t.Fatal(err)
			}

			buf := make([]byte, 1024)

			if _, err := out.Read(buf); err != nil && err != io.EOF {
				t.Fatal(err)
			}

			startGroup.Done()

			if !strings.Contains(string(buf), "hello") {
				t.Fatalf("unexpected output %s expected hello\n", string(buf))
			}
		}()
	}

	select {
	case <-startDone:
	case <-time.After(attachWait):
		t.Fatalf("Attaches did not initialize properly")
	}

	cmd := exec.Command(dockerBinary, "kill", "attacher")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	select {
	case <-endDone:
	case <-time.After(attachWait):
		t.Fatalf("Attaches did not finish properly")
	}

	logDone("attach - multiple attach")
}

func TestAttachTtyWithoutStdin(t *testing.T) {
	defer deleteAllContainers()

	cmd := exec.Command(dockerBinary, "run", "-d", "-ti", "busybox")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("failed to start container: %v (%v)", out, err)
	}

	id := strings.TrimSpace(out)
	if err := waitRun(id); err != nil {
		t.Fatal(err)
	}

	defer func() {
		cmd := exec.Command(dockerBinary, "kill", id)
		if out, _, err := runCommandWithOutput(cmd); err != nil {
			t.Fatalf("failed to kill container: %v (%v)", out, err)
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)

		cmd := exec.Command(dockerBinary, "attach", id)
		if _, err := cmd.StdinPipe(); err != nil {
			t.Fatal(err)
		}

		expected := "cannot enable tty mode"
		if out, _, err := runCommandWithOutput(cmd); err == nil {
			t.Fatal("attach should have failed")
		} else if !strings.Contains(out, expected) {
			t.Fatalf("attach failed with error %q: expected %q", out, expected)
		}
	}()

	select {
	case <-done:
	case <-time.After(attachWait):
		t.Fatal("attach is running but should have failed")
	}

	logDone("attach - forbid piped stdin to tty enabled container")
}

func TestAttachDisconnect(t *testing.T) {
	defer deleteAllContainers()
	out, _, _ := dockerCmd(t, "run", "-di", "busybox", "/bin/cat")
	id := strings.TrimSpace(out)

	cmd := exec.Command(dockerBinary, "attach", id)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()

	if _, err := stdin.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	out, err = bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Fatalf("exepected 'hello', got %q", out)
	}

	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}

	// Expect container to still be running after stdin is closed
	running, err := inspectField(id, "State.Running")
	if err != nil {
		t.Fatal(err)
	}
	if running != "true" {
		t.Fatal("exepected container to still be running")
	}

	logDone("attach - disconnect")
}
