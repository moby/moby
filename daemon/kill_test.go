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

	err := daemon.killWithSignal(ctr, syscall.SIGKILL)
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
