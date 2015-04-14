// +build !windows

package main

import (
	"bufio"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/stringid"
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

	id := strings.TrimSpace(out)
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
	if err := waitRun(name); err != nil {
		t.Fatal(err)
	}
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

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	bytes := make([]byte, 10)
	var nBytes int
	readErr := make(chan error, 1)

	go func() {
		time.Sleep(500 * time.Millisecond)
		cpty.Write([]byte("\n"))
		time.Sleep(500 * time.Millisecond)

		nBytes, err = cpty.Read(bytes)
		cpty.Close()
		readErr <- err
	}()

	select {
	case err := <-readErr:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for attach read")
	}

	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(bytes[:nBytes]), "/ #") {
		t.Fatalf("failed to get a new prompt. got %s", string(bytes[:nBytes]))
	}

	logDone("attach - reconnect after detaching")
}

// TestAttachDetach checks that attach in tty mode can be detached using the long container ID
func TestAttachDetach(t *testing.T) {
	out, _, _ := dockerCmd(t, "run", "-itd", "busybox", "cat")
	id := strings.TrimSpace(out)
	if err := waitRun(id); err != nil {
		t.Fatal(err)
	}

	cpty, tty, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer cpty.Close()

	cmd := exec.Command(dockerBinary, "attach", id)
	cmd.Stdin = tty
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := waitRun(id); err != nil {
		t.Fatalf("error waiting for container to start: %v", err)
	}

	if _, err := cpty.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	out, err = bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Fatalf("exepected 'hello', got %q", out)
	}

	// escape sequence
	if _, err := cpty.Write([]byte{16}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := cpty.Write([]byte{17}); err != nil {
		t.Fatal(err)
	}

	ch := make(chan struct{})
	go func() {
		cmd.Wait()
		ch <- struct{}{}
	}()

	running, err := inspectField(id, "State.Running")
	if err != nil {
		t.Fatal(err)
	}
	if running != "true" {
		t.Fatal("exepected container to still be running")
	}

	go func() {
		dockerCmd(t, "kill", id)
	}()

	select {
	case <-ch:
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timed out waiting for container to exit")
	}

	logDone("attach - detach")
}

// TestAttachDetachTruncatedID checks that attach in tty mode can be detached
func TestAttachDetachTruncatedID(t *testing.T) {
	out, _, _ := dockerCmd(t, "run", "-itd", "busybox", "cat")
	id := stringid.TruncateID(strings.TrimSpace(out))
	if err := waitRun(id); err != nil {
		t.Fatal(err)
	}

	cpty, tty, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer cpty.Close()

	cmd := exec.Command(dockerBinary, "attach", id)
	cmd.Stdin = tty
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	if _, err := cpty.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	out, err = bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Fatalf("exepected 'hello', got %q", out)
	}

	// escape sequence
	if _, err := cpty.Write([]byte{16}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := cpty.Write([]byte{17}); err != nil {
		t.Fatal(err)
	}

	ch := make(chan struct{})
	go func() {
		cmd.Wait()
		ch <- struct{}{}
	}()

	running, err := inspectField(id, "State.Running")
	if err != nil {
		t.Fatal(err)
	}
	if running != "true" {
		t.Fatal("exepected container to still be running")
	}

	go func() {
		dockerCmd(t, "kill", id)
	}()

	select {
	case <-ch:
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timed out waiting for container to exit")
	}

	logDone("attach - detach truncated ID")
}
