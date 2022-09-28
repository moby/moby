//go:build !windows
// +build !windows

package main

import (
	"bufio"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"gotest.tools/v3/assert"
)

// #9860 Make sure attach ends when container ends (with no errors)
func (s *DockerCLIAttachSuite) TestAttachClosedOnContainerStop(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon)

	out, _ := dockerCmd(c, "run", "-dti", "busybox", "/bin/sh", "-c", `trap 'exit 0' SIGTERM; while true; do sleep 1; done`)

	id := strings.TrimSpace(out)
	assert.NilError(c, waitRun(id))

	pty, tty, err := pty.Open()
	assert.NilError(c, err)

	attachCmd := exec.Command(dockerBinary, "attach", id)
	attachCmd.Stdin = tty
	attachCmd.Stdout = tty
	attachCmd.Stderr = tty
	err = attachCmd.Start()
	assert.NilError(c, err)

	errChan := make(chan error, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		defer close(errChan)
		// Container is waiting for us to signal it to stop
		dockerCmd(c, "stop", id)
		// And wait for the attach command to end
		errChan <- attachCmd.Wait()
	}()

	// Wait for the docker to end (should be done by the
	// stop command in the go routine)
	dockerCmd(c, "wait", id)

	select {
	case err := <-errChan:
		tty.Close()
		out, _ := io.ReadAll(pty)
		assert.Assert(c, err == nil, "out: %v", string(out))
	case <-time.After(attachWait):
		c.Fatal("timed out without attach returning")
	}
}

func (s *DockerCLIAttachSuite) TestAttachAfterDetach(c *testing.T) {
	name := "detachtest"

	cpty, tty, err := pty.Open()
	assert.NilError(c, err, "Could not open pty: %v", err)
	cmd := exec.Command(dockerBinary, "run", "-ti", "--name", name, "busybox")
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty

	cmdExit := make(chan error, 1)
	go func() {
		cmdExit <- cmd.Run()
		close(cmdExit)
	}()

	assert.Assert(c, waitRun(name) == nil)

	cpty.Write([]byte{16})
	time.Sleep(100 * time.Millisecond)
	cpty.Write([]byte{17})

	select {
	case <-cmdExit:
	case <-time.After(5 * time.Second):
		c.Fatal("timeout while detaching")
	}

	cpty, tty, err = pty.Open()
	assert.NilError(c, err, "Could not open pty: %v", err)

	cmd = exec.Command(dockerBinary, "attach", name)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty

	err = cmd.Start()
	assert.NilError(c, err)
	defer cmd.Process.Kill()

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
		assert.NilError(c, err)
	case <-time.After(2 * time.Second):
		c.Fatal("timeout waiting for attach read")
	}

	assert.Assert(c, strings.Contains(string(bytes[:nBytes]), "/ #"))
}

// TestAttachDetach checks that attach in tty mode can be detached using the long container ID
func (s *DockerCLIAttachSuite) TestAttachDetach(c *testing.T) {
	out, _ := dockerCmd(c, "run", "-itd", "busybox", "cat")
	id := strings.TrimSpace(out)
	assert.NilError(c, waitRun(id))

	cpty, tty, err := pty.Open()
	assert.NilError(c, err)
	defer cpty.Close()

	cmd := exec.Command(dockerBinary, "attach", id)
	cmd.Stdin = tty
	stdout, err := cmd.StdoutPipe()
	assert.NilError(c, err)
	defer stdout.Close()
	err = cmd.Start()
	assert.NilError(c, err)
	assert.NilError(c, waitRun(id))

	_, err = cpty.Write([]byte("hello\n"))
	assert.NilError(c, err)
	out, err = bufio.NewReader(stdout).ReadString('\n')
	assert.NilError(c, err)
	assert.Equal(c, strings.TrimSpace(out), "hello")

	// escape sequence
	_, err = cpty.Write([]byte{16})
	assert.NilError(c, err)
	time.Sleep(100 * time.Millisecond)
	_, err = cpty.Write([]byte{17})
	assert.NilError(c, err)

	ch := make(chan struct{})
	go func() {
		cmd.Wait()
		close(ch)
	}()

	select {
	case <-ch:
	case <-time.After(1 * time.Second):
		c.Fatal("timed out waiting for container to exit")
	}

	running := inspectField(c, id, "State.Running")
	assert.Equal(c, running, "true") // container should be running
}
