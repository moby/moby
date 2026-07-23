package daemon

import (
	"context"
	"syscall"
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	cerrdefs "github.com/containerd/errdefs"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/events"
	libcontainerdtypes "github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
	"gotest.tools/v3/assert"
)

type notFoundKillTask struct {
	libcontainerdtypes.Task
	deleteCalled chan struct{}
}

type killContextKey struct{}

type contextKillTask struct {
	libcontainerdtypes.Task
	killContext chan context.Context
}

func (t *contextKillTask) Pid() uint32 {
	return 1
}

func (t *contextKillTask) Kill(ctx context.Context, _ syscall.Signal) error {
	t.killContext <- ctx
	return nil
}

func (t *notFoundKillTask) Pid() uint32 {
	return 1
}

func (t *notFoundKillTask) Kill(context.Context, syscall.Signal) error {
	return cerrdefs.ErrNotFound
}

func (t *notFoundKillTask) Delete(ctx context.Context) (*containerd.ExitStatus, error) {
	close(t.deleteCalled)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestKillWithSignalWaitsIndefinitelyForDelayedExit(t *testing.T) {
	task := &notFoundKillTask{deleteCalled: make(chan struct{})}
	ctr := container.NewBaseContainer(t.Name(), t.TempDir())
	stopTimeout := -1
	ctr.Config = &containertypes.Config{StopTimeout: &stopTimeout}
	ctr.HostConfig = &containertypes.HostConfig{}
	ctr.Lock()
	ctr.State.SetRunning(nil, task, time.Now())
	ctr.Unlock()

	daemon := &Daemon{
		EventsService: events.New(),
		shutdown:      true,
	}

	err := daemon.killWithSignal(t.Context(), ctr, syscall.SIGKILL)
	assert.NilError(t, err)

	select {
	case <-task.deleteCalled:
		t.Fatal("fallback exit handling ran before the delayed exit event")
	case <-time.After(100 * time.Millisecond):
	}

	ctr.Lock()
	ctr.State.SetStopped(&container.ExitStatus{})
	ctr.Unlock()
}

func TestKillWithSignalDetachesContext(t *testing.T) {
	task := &contextKillTask{killContext: make(chan context.Context, 1)}
	ctr := container.NewBaseContainer(t.Name(), t.TempDir())
	ctr.Config = &containertypes.Config{}
	ctr.HostConfig = &containertypes.HostConfig{}
	ctr.Lock()
	ctr.State.SetRunning(nil, task, time.Now())
	ctr.Unlock()

	daemon := &Daemon{
		EventsService: events.New(),
		shutdown:      true,
	}

	const contextValue = "value"
	ctx := context.WithValue(t.Context(), killContextKey{}, contextValue)
	ctx, cancel := context.WithCancel(ctx)
	cancel()

	err := daemon.killWithSignal(ctx, ctr, syscall.SIGKILL)
	assert.NilError(t, err)

	killCtx := <-task.killContext
	assert.NilError(t, killCtx.Err())
	assert.Equal(t, killCtx.Value(killContextKey{}), contextValue)
}
