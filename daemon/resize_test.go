//go:build linux
// +build linux

package daemon

import (
	"context"
	"testing"

	"github.com/docker/docker/container"
	"github.com/docker/docker/libcontainerd/types"
	"gotest.tools/v3/assert"
)

// This test simply verify that when a wrong ID used, a specific error should be returned for exec resize.
func TestExecResizeNoSuchExec(t *testing.T) {
	n := "TestExecResize"
	d := &Daemon{
		execCommands: container.NewExecStore(),
	}
	c := &container.Container{
		ExecCommands: container.NewExecStore(),
	}
	ec := &container.ExecConfig{
		ID:        n,
		Container: c,
	}
	d.registerExecCommand(c, ec)
	err := d.ContainerExecResize("nil", 24, 8)
	assert.ErrorContains(t, err, "No such exec instance")
}

type execResizeMockProcess struct {
	types.Process
	Width, Height int
}

func (p *execResizeMockProcess) Resize(ctx context.Context, width, height uint32) error {
	p.Width = int(width)
	p.Height = int(height)
	return nil
}

// This test is to make sure that when exec context is ready, resize should call ResizeTerminal via containerd client.
func TestExecResize(t *testing.T) {
	n := "TestExecResize"
	width := 24
	height := 8
	mp := &execResizeMockProcess{}
	d := &Daemon{
		execCommands: container.NewExecStore(),
		containers:   container.NewMemoryStore(),
	}
	c := &container.Container{
		ID:           n,
		ExecCommands: container.NewExecStore(),
		State:        &container.State{Running: true},
	}
	ec := &container.ExecConfig{
		ID:        n,
		Container: c,
		Process:   mp,
		Started:   make(chan struct{}),
	}
	close(ec.Started)
	d.containers.Add(n, c)
	d.registerExecCommand(c, ec)
	err := d.ContainerExecResize(n, height, width)
	assert.NilError(t, err)
	assert.Equal(t, mp.Width, width)
	assert.Equal(t, mp.Height, height)
}

// This test is to make sure that when exec context is not ready, a timeout error should happen.
// TODO: the expect running time for this test is 10s, which would be too long for unit test.
func TestExecResizeTimeout(t *testing.T) {
	n := "TestExecResize"
	width := 24
	height := 8
	mp := &execResizeMockProcess{}
	d := &Daemon{
		execCommands: container.NewExecStore(),
		containers:   container.NewMemoryStore(),
	}
	c := &container.Container{
		ID:           n,
		ExecCommands: container.NewExecStore(),
		State:        &container.State{Running: true},
	}
	ec := &container.ExecConfig{
		ID:        n,
		Container: c,
		Process:   mp,
		Started:   make(chan struct{}),
	}
	d.containers.Add(n, c)
	d.registerExecCommand(c, ec)
	err := d.ContainerExecResize(n, height, width)
	assert.ErrorContains(t, err, "timeout waiting for exec session ready")
}
