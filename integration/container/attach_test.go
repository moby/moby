// +build !windows

package container

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/poll"
)

// #9860 Make sure attach ends when container ends (with no errors)
func TestAttachClosedOnContainerStop(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()

	cID := container.Run(ctx, t, client, container.WithTty(true),
		container.WithCmd("/bin/sh", "-c", `trap 'exit 0' SIGTERM; while true; do sleep 1; done`),
		container.WithStdin(true),
	)
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	attach, err := client.ContainerAttach(ctx, cID,
		types.ContainerAttachOptions{
			Stream: true,
			Stdin:  true,
			Stdout: true,
			Stderr: true,
		},
	)
	assert.NilError(t, err)
	defer attach.Close()

	errCh := make(chan error)
	go func() {
		defer close(errCh)
		time.Sleep(300 * time.Millisecond)
		err = client.ContainerStop(ctx, cID, nil)
		assert.NilError(t, err)
		_, err = io.Copy(&bytes.Buffer{}, attach.Reader)
		errCh <- err
	}()

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond))

	select {
	case err = <-errCh:
		assert.NilError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for attach channel to be closed")
	}
}

func TestAttachAfterDetach(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()

	cID := container.Run(ctx, t, client, container.WithTty(true),
		container.WithCmd("sh"),
		container.WithStdin(true),
	)

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	attach, err := client.ContainerAttach(ctx, cID,
		types.ContainerAttachOptions{
			Stream: true,
			Stdin:  true,
			Stdout: true,
			Stderr: true,
		},
	)
	assert.NilError(t, err)

	errCh := make(chan error)
	go func() {
		defer close(errCh)
		_, err := io.Copy(&bytes.Buffer{}, attach.Reader)
		errCh <- err
	}()

	_, err = attach.Conn.Write([]byte{16})
	assert.NilError(t, err)
	time.Sleep(100 * time.Millisecond)
	_, err = attach.Conn.Write([]byte{17})
	assert.NilError(t, err)

	select {
	case err := <-errCh:
		assert.NilError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out while detaching")
	}
	attach.Close()

	attach, err = client.ContainerAttach(ctx, cID,
		types.ContainerAttachOptions{
			Stream: true,
			Stdin:  true,
			Stdout: true,
			Stderr: true,
		},
	)
	assert.NilError(t, err)
	defer attach.Close()

	buf := make([]byte, 10)
	var nBytes int
	errCh = make(chan error)
	go func() {
		defer close(errCh)
		_, err := attach.Conn.Write([]byte("\n"))
		assert.NilError(t, err)
		time.Sleep(500 * time.Millisecond)

		nBytes, err = attach.Reader.Read(buf)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		assert.NilError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for attach read")
	}

	assert.Assert(t, is.Contains(string(buf[:nBytes]), "/ #"))
}

// TestAttachDetach checks that attach in tty mode can be detached using the long container ID
func TestAttachDetach(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()

	cID := container.Run(ctx, t, client, container.WithTty(true),
		container.WithCmd("cat"),
		container.WithStdin(true),
	)
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	attach, err := client.ContainerAttach(ctx, cID,
		types.ContainerAttachOptions{
			Stream: true,
			Stdin:  true,
			Stdout: true,
			Stderr: true,
		},
	)
	assert.NilError(t, err)
	defer attach.Close()

	_, err = attach.Conn.Write([]byte("hello\n"))
	assert.NilError(t, err)
	got, err := bufio.NewReader(attach.Reader).ReadString('\n')
	assert.NilError(t, err)
	assert.Check(t, is.Equal("hello", strings.TrimSpace(got)))

	// escape sequence
	_, err = attach.Conn.Write([]byte{16})
	assert.NilError(t, err)
	time.Sleep(100 * time.Millisecond)
	_, err = attach.Conn.Write([]byte{17})
	assert.NilError(t, err)

	errCh := make(chan error)
	go func() {
		defer close(errCh)
		_, err := io.Copy(&bytes.Buffer{}, attach.Reader)
		errCh <- err
	}()

	select {
	case err = <-errCh:
		assert.NilError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for attach channel to be closed")
	}

	inspect, err := client.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Assert(t, is.Equal(true, inspect.State.Running))
}
