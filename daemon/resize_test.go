//go:build linux
// +build linux

package daemon

import (
	"context"
	"testing"

	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/exec"
	"gotest.tools/v3/assert"
)

// This test simply verify that when a wrong ID used, a specific error should be returned for exec resize.
func TestExecResizeNoSuchExec(t *testing.T) {
	n := "TestExecResize"
	d := &Daemon{
		execCommands: exec.NewStore(),
	}
	c := &container.Container{
		ExecCommands: exec.NewStore(),
	}
	ec := &exec.Config{
		ID: n,
	}
	d.registerExecCommand(c, ec)
	err := d.ContainerExecResize("nil", 24, 8)
	assert.ErrorContains(t, err, "No such exec instance")
}

type execResizeMockContainerdClient struct {
	MockContainerdClient
	ProcessID   string
	ContainerID string
	Width       int
	Height      int
}

func (c *execResizeMockContainerdClient) ResizeTerminal(ctx context.Context, containerID, processID string, width, height int) error {
	c.ProcessID = processID
	c.ContainerID = containerID
	c.Width = width
	c.Height = height
	return nil
}

// This test is to make sure that when exec context is ready, resize should call ResizeTerminal via containerd client.
func TestExecResize(t *testing.T) {
	n := "TestExecResize"
	width := 24
	height := 8
	ec := &exec.Config{
		ID:          n,
		ContainerID: n,
		Started:     make(chan struct{}),
	}
	close(ec.Started)
	mc := &execResizeMockContainerdClient{}
	d := &Daemon{
		execCommands: exec.NewStore(),
		containerd:   mc,
		containers:   container.NewMemoryStore(),
	}
	c := &container.Container{
		ExecCommands: exec.NewStore(),
		State:        &container.State{Running: true},
	}
	d.containers.Add(n, c)
	d.registerExecCommand(c, ec)
	err := d.ContainerExecResize(n, height, width)
	assert.NilError(t, err)
	assert.Equal(t, mc.Width, width)
	assert.Equal(t, mc.Height, height)
	assert.Equal(t, mc.ProcessID, n)
	assert.Equal(t, mc.ContainerID, n)
}

// This test is to make sure that when exec context is not ready, a timeout error should happen.
// TODO: the expect running time for this test is 10s, which would be too long for unit test.
func TestExecResizeTimeout(t *testing.T) {
	n := "TestExecResize"
	width := 24
	height := 8
	ec := &exec.Config{
		ID:          n,
		ContainerID: n,
		Started:     make(chan struct{}),
	}
	mc := &execResizeMockContainerdClient{}
	d := &Daemon{
		execCommands: exec.NewStore(),
		containerd:   mc,
		containers:   container.NewMemoryStore(),
	}
	c := &container.Container{
		ExecCommands: exec.NewStore(),
		State:        &container.State{Running: true},
	}
	d.containers.Add(n, c)
	d.registerExecCommand(c, ec)
	err := d.ContainerExecResize(n, height, width)
	assert.ErrorContains(t, err, "timeout waiting for exec session ready")
}
