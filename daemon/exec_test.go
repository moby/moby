//go:build linux

package daemon

import (
	"context"
	"testing"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/versions"
	"github.com/moby/moby/v2/daemon/container"
	libcontainerdtypes "github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
	"github.com/moby/moby/v2/daemon/server/httputils"
	"github.com/moby/sys/signal"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

type mockContainerd struct {
	// MockContainerdClient
	libcontainerdtypes.Client
	calledCtx         *context.Context
	calledContainerID *string
	calledID          *string
	calledSig         *int
}

func (cd *mockContainerd) SignalProcess(ctx context.Context, containerID, id string, sig int) error {
	cd.calledCtx = &ctx
	cd.calledContainerID = &containerID
	cd.calledID = &id
	cd.calledSig = &sig
	return nil
}

func TestContainerExecSignalNoSuchExec(t *testing.T) {
	ctx := context.Background()
	version := httputils.VersionFromContext(ctx)
	skip.If(t, versions.LessThan(version, "1.42"), "skip test from new feature")

	mock := mockContainerd{}
	d := &Daemon{
		execCommands: container.NewExecStore(),
		containerd:   &mock,
	}

	err := d.ContainerExecSignal(ctx, "nil", types.ExecSignalConfig{Signal: "TERM"})
	assert.ErrorContains(t, err, "No such exec instance")
	assert.Assert(t, is.Nil(mock.calledCtx))
	assert.Assert(t, is.Nil(mock.calledContainerID))
	assert.Assert(t, is.Nil(mock.calledID))
	assert.Assert(t, is.Nil(mock.calledSig))
}

func TestContainerExecSignal(t *testing.T) {
	ctx := context.Background()
	version := httputils.VersionFromContext(ctx)
	skip.If(t, versions.LessThan(version, "1.42"), "skip test from new feature")

	mock := mockContainerd{}
	c := &container.Container{
		ExecCommands: container.NewExecStore(),
		State:        &container.State{Running: true},
	}
	ec := &container.ExecConfig{
		ID: "exec",
		// ContainerID: "container",
		Started: make(chan struct{}),
	}
	d := &Daemon{
		execCommands: container.NewExecStore(),
		containers:   container.NewMemoryStore(),
		containerd:   &mock,
	}
	d.containers.Add("container", c)
	d.registerExecCommand(c, ec)

	err := d.ContainerExecSignal(ctx, "exec", types.ExecSignalConfig{Signal: "TERM"})
	assert.NilError(t, err)
	assert.Equal(t, *mock.calledCtx, ctx)
	assert.Equal(t, *mock.calledContainerID, "container")
	assert.Equal(t, *mock.calledID, "exec")
	assert.Equal(t, *mock.calledSig, int(signal.SignalMap["TERM"]))
}
