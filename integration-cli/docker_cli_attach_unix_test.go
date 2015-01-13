package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kr/pty"
)

// #9860
func TestAttachClosedOnContainerStop(t *testing.T) {
	defer deleteAllContainers()

	cmd := exec.Command(dockerBinary, "run", "-dti", "busybox", "sleep", "2")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("failed to start container: %v (%v)", out, err)
	}

	id := stripTrailingCharacters(out)
	if err := waitRun(id); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		_, tty, err := pty.Open()
		if err != nil {
			t.Fatalf("could not open pty: %v", err)
		}
		attachCmd := exec.Command(dockerBinary, "attach", id)
		attachCmd.Stdin = tty
		attachCmd.Stdout = tty
		attachCmd.Stderr = tty

		if err := attachCmd.Run(); err != nil {
			t.Fatalf("attach returned error %s", err)
		}
	}()

	waitCmd := exec.Command(dockerBinary, "wait", id)
	if out, _, err = runCommandWithOutput(waitCmd); err != nil {
		t.Fatalf("error thrown while waiting for container: %s, %v", out, err)
	}
	select {
	case <-done:
	case <-time.After(attachWait):
		t.Fatal("timed out without attach returning")
	}

	logDone("attach - return after container finished")
}

func TestAttachAfterDetach(t *testing.T) {
	defer deleteAllContainers()

	name := "detachtest"

	cpty, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("Could not open pty: %v", err)
	}
	cmd := exec.Command(dockerBinary, "run", "-ti", "--name", name, "busybox")
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty

	detached := make(chan struct{})
	go func() {
		if err := cmd.Run(); err != nil {
			t.Fatalf("attach returned error %s", err)
		}
		close(detached)
	}()

	time.Sleep(500 * time.Millisecond)
	cpty.Write([]byte{16})
	time.Sleep(100 * time.Millisecond)
	cpty.Write([]byte{17})

	<-detached

	cpty, tty, err = pty.Open()
	if err != nil {
		t.Fatalf("Could not open pty: %v", err)
	}

	cmd = exec.Command(dockerBinary, "attach", name)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty

	go func() {
		if err := cmd.Run(); err != nil {
			t.Fatalf("attach returned error %s", err)
		}
		cpty.Close() // unblocks the reader in case of a failure
	}()

	time.Sleep(500 * time.Millisecond)
	cpty.Write([]byte("\n"))
	time.Sleep(500 * time.Millisecond)
	bytes := make([]byte, 10)

	n, err := cpty.Read(bytes)

	if err != nil {
		t.Fatalf("prompt read failed: %v", err)
	}

	if !strings.Contains(string(bytes[:n]), "/ #") {
		t.Fatalf("failed to get a new prompt. got %s", string(bytes[:n]))
	}

	cpty.Write([]byte("exit\n"))

	logDone("attach - reconnect after detaching")
}
