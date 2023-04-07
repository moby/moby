package container // import "github.com/docker/docker/integration/container"

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cpuguy83/pipes"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/streams"
	"github.com/docker/docker/pkg/stringid"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	intctr "github.com/docker/docker/integration/internal/container"
)

func TestAttachWithTTY(t *testing.T) {
	testAttach(t, true, types.MediaTypeRawStream)
}

func TestAttachWithoutTTy(t *testing.T) {
	testAttach(t, false, types.MediaTypeMultiplexedStream)
}

func testAttach(t *testing.T, tty bool, expected string) {
	defer setupTest(t)()
	client := testEnv.APIClient()

	resp, err := client.ContainerCreate(context.Background(),
		&container.Config{
			Image: "busybox",
			Cmd:   []string{"echo", "hello"},
			Tty:   tty,
		},
		&container.HostConfig{},
		&network.NetworkingConfig{},
		nil,
		"",
	)
	assert.NilError(t, err)
	container := resp.ID
	defer client.ContainerRemove(context.Background(), container, types.ContainerRemoveOptions{
		Force: true,
	})

	attach, err := client.ContainerAttach(context.Background(), container, types.ContainerAttachOptions{
		Stdout: true,
		Stderr: true,
	})
	assert.NilError(t, err)
	mediaType, ok := attach.MediaType()
	assert.Check(t, ok)
	assert.Check(t, mediaType == expected)
}

func openFifo(t *testing.T, p string, flag int, perm os.FileMode) <-chan pipes.OpenFifoResult {
	t.Helper()

	waiter, err := pipes.AsyncOpenFifo(p, flag, perm)
	assert.NilError(t, err)
	t.Cleanup(func() {
		select {
		case res := <-waiter:
			assert.Check(t, cmp.Nil(res.Err))
			if res.W != nil {
				res.W.Close()
			}
			if res.R != nil {
				res.R.Close()
			}
		default:
		}
	})
	return waiter
}

func TestAttachStreams(t *testing.T) {
	defer setupTest(t)()
	client := testEnv.APIClient()
	defer client.Close()

	dir := t.TempDir()
	stdinP := filepath.Join(dir, "stdin")
	stdoutP := filepath.Join(dir, "stdout")
	stderrP := filepath.Join(dir, "stderr")

	stdinWaiter := openFifo(t, stdinP, unix.O_WRONLY|unix.O_CREAT, 0600)
	stdoutWaiter := openFifo(t, stdoutP, unix.O_RDONLY|unix.O_CREAT, 0600)
	stderrWaiter := openFifo(t, stderrP, unix.O_RDONLY|unix.O_CREAT, 0600)

	ctx := context.Background()

	stdinID := stringid.GenerateRandomID()
	resp, err := client.StreamCreate(ctx, stdinID, streams.Spec{
		Protocol: streams.ProtocolPipe,
		PipeConfig: &streams.PipeConfig{
			Path: stdinP,
		},
	})
	assert.NilError(t, err)
	defer client.StreamDelete(ctx, resp.ID)

	stdoutID := stringid.GenerateRandomID()
	resp, err = client.StreamCreate(ctx, stdoutID, streams.Spec{
		Protocol: streams.ProtocolPipe,
		PipeConfig: &streams.PipeConfig{
			Path: stdoutP,
		},
	})
	assert.NilError(t, err)
	defer client.StreamDelete(ctx, resp.ID)

	stderrID := stringid.GenerateRandomID()
	resp, err = client.StreamCreate(ctx, stderrID, streams.Spec{
		Protocol: streams.ProtocolPipe,
		PipeConfig: &streams.PipeConfig{
			Path: stderrP,
		},
	})
	assert.NilError(t, err)
	defer client.StreamDelete(ctx, resp.ID)

	id := intctr.Run(ctx, t, client, func(cfg *intctr.TestContainerConfig) {
		cfg.Config.OpenStdin = true
		cfg.Config.AttachStdin = true
		cfg.Config.AttachStdout = true
		cfg.Config.AttachStderr = true
		cfg.Config.Cmd = []string{"cat", "-"}
	})

	chErr := make(chan error, 1)
	go func() {
		err = client.ContainerAttachStreams(ctx, id, types.AttachStreamConfig{
			Stdin:  stdinID,
			Stdout: stdoutID,
			Stderr: stderrID,
		})
		chErr <- err
	}()

	waitForPipe := func(waiter <-chan pipes.OpenFifoResult) pipes.OpenFifoResult {
		t.Helper()
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()

		select {
		case res := <-waiter:
			assert.NilError(t, res.Err)
			return res
		case err := <-chErr:
			assert.NilError(t, err, "attach failed")
		case <-timer.C:
			t.Fatal("timed out waiting for pipe to open")
		}
		panic("unreachable")
	}

	stdin := waitForPipe(stdinWaiter)
	defer stdin.W.Close()

	stdout := waitForPipe(stdoutWaiter)
	defer stdout.R.Close()

	stderr := waitForPipe(stderrWaiter)
	defer stderr.R.Close()

	payload := []byte("hello\n")
	_, err = stdin.W.Write(payload)
	assert.NilError(t, err)
	stdin.W.Close()

	buf := make([]byte, len(payload))
	// n, err := io.ReadFull(stdout.R, buf)
	n, err := stdout.R.Read(buf)
	assert.NilError(t, err)
	assert.Assert(t, bytes.Equal(payload[:n], buf))
}
