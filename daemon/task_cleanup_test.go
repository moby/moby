package daemon

import (
	"context"
	"sync"
	"testing"

	containerd "github.com/containerd/containerd/v2/client"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/events"
	libcontainerdtypes "github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
	"github.com/moby/moby/v2/daemon/internal/restartmanager"
	"gotest.tools/v3/assert"
)

type blockedTaskCleaner struct {
	libcontainerdtypes.Client
	attempts       chan uint32
	results        chan error
	loaded         libcontainerdtypes.Container
	canceled       chan uint32
	finishCanceled chan struct{}
}

func (c *blockedTaskCleaner) LoadContainer(context.Context, string) (libcontainerdtypes.Container, error) {
	return c.loaded, nil
}

func (c *blockedTaskCleaner) CleanupExitedTask(ctx context.Context, _ string, pid uint32) error {
	c.attempts <- pid
	select {
	case err := <-c.results:
		return err
	case <-ctx.Done():
		if c.canceled == nil {
			return ctx.Err()
		}
		c.canceled <- pid
		<-c.finishCanceled
		return nil
	}
}

func TestTaskCleanupSharesAttemptsAndAllowsRetry(t *testing.T) {
	cleaner := &blockedTaskCleaner{attempts: make(chan uint32, 1), results: make(chan error, 1)}
	d := &Daemon{containerd: cleaner, taskCleanupContext: t.Context()}
	c := container.NewBaseContainer("container", t.TempDir())

	c.Lock()
	c.TaskCleanupPID = 123
	first := d.startTaskCleanup(c)
	assert.Equal(t, d.startTaskCleanup(c), first)
	c.Unlock()
	assert.Equal(t, <-cleaner.attempts, uint32(123))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	c.Lock()
	err := d.awaitTaskCleanup(ctx, c)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, c.TaskCleanup, first)
	c.Unlock()

	cleaner.results <- context.DeadlineExceeded
	<-first.Done
	assert.ErrorIs(t, first.Err, context.DeadlineExceeded)

	c.Lock()
	assert.Equal(t, c.TaskCleanupPID, uint32(123))
	second := d.startTaskCleanup(c)
	c.Unlock()
	assert.Check(t, second != first)
	assert.Equal(t, <-cleaner.attempts, uint32(123))
	cleaner.results <- context.DeadlineExceeded
	<-second.Done
	assert.ErrorIs(t, second.Err, context.DeadlineExceeded)
}

func TestContainerStartRejectsSupersededRestart(t *testing.T) {
	c := container.NewBaseContainer("container", t.TempDir())
	c.Config = &containertypes.Config{}
	c.HostConfig = &containertypes.HostConfig{}

	c.State.SetRestarting(&container.ExitStatus{})
	old := c.RestartManager()
	c.ResetRestartManager(false)
	_ = c.RestartManager()

	d := &Daemon{}
	err := d.containerStart(t.Context(), nil, c, "", "", old)
	assert.ErrorIs(t, err, restartmanager.ErrRestartCanceled)
}

func TestAwaitTaskCleanupRechecksPendingTask(t *testing.T) {
	d := &Daemon{}
	c := container.NewBaseContainer("container", t.TempDir())
	first := &container.TaskCleanupAttempt{Done: make(chan struct{})}
	second := &container.TaskCleanupAttempt{Done: make(chan struct{}), Err: context.DeadlineExceeded}
	c.TaskCleanupPID, c.TaskCleanup = 123, first

	waiting := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		c.Lock()
		defer c.Unlock()
		close(waiting)
		result <- d.awaitTaskCleanup(t.Context(), c)
	}()

	<-waiting
	c.Lock()
	// Complete the first attempt and record a newer one before the waiter
	// can reacquire the lock.
	c.TaskCleanupPID, c.TaskCleanup = 456, second
	close(first.Done)
	close(second.Done)
	c.Unlock()
	assert.ErrorIs(t, <-result, context.DeadlineExceeded)
}

type replacementTaskContainer struct {
	libcontainerdtypes.Container
	task libcontainerdtypes.Task
}

func (c replacementTaskContainer) Task(context.Context) (libcontainerdtypes.Task, error) {
	return c.task, nil
}

type replacementTask struct {
	libcontainerdtypes.Task
	pid      uint32
	exitCode uint32
}

func (task replacementTask) Pid() uint32 { return task.pid }

func (task replacementTask) Status(context.Context) (containerd.Status, error) {
	return containerd.Status{Status: containerd.Stopped, ExitStatus: task.exitCode}, nil
}

func (task replacementTask) Delete(context.Context) (*containerd.ExitStatus, error) {
	return nil, context.DeadlineExceeded
}

func TestExternalReplacementSupersedesTaskCleanup(t *testing.T) {
	const oldPID, newPID, exitCode = uint32(123), uint32(456), uint32(23)
	store := container.NewMemoryStore()
	replica, err := container.NewViewDB()
	assert.NilError(t, err)
	c := container.NewBaseContainer("container", t.TempDir())
	c.Config = &containertypes.Config{}
	c.HostConfig = &containertypes.HostConfig{}
	c.State.SetRestarting(&container.ExitStatus{})
	c.TaskCleanupPID = oldPID
	store.Add(c.ID, c)

	cleaner := &blockedTaskCleaner{
		attempts:       make(chan uint32, 1),
		results:        make(chan error, 1),
		loaded:         replacementTaskContainer{task: replacementTask{pid: newPID, exitCode: exitCode}},
		canceled:       make(chan uint32, 1),
		finishCanceled: make(chan struct{}),
	}
	d := &Daemon{
		containerd:         cleaner,
		containers:         store,
		containersReplica:  replica,
		EventsService:      events.New(),
		taskCleanupContext: t.Context(),
	}
	t.Cleanup(func() { _ = d.EventsService.Close() })
	finishCanceled := sync.OnceFunc(func() { close(cleaner.finishCanceled) })
	t.Cleanup(finishCanceled)

	c.Lock()
	old := d.startTaskCleanup(c)
	c.Unlock()
	assert.Equal(t, <-cleaner.attempts, oldPID)

	event := libcontainerdtypes.EventInfo{ContainerID: c.ID, ProcessID: c.ID, Pid: newPID}
	assert.NilError(t, d.ProcessEvent(c.ID, libcontainerdtypes.EventStart, event))
	assert.Equal(t, c.State.GetPID(), int(newPID))
	assert.Equal(t, <-cleaner.canceled, oldPID)

	event.ExitCode = exitCode
	assert.NilError(t, d.ProcessEvent(c.ID, libcontainerdtypes.EventExit, event))
	assert.Equal(t, <-cleaner.attempts, newPID)

	// The superseded RPC can still complete successfully after the new exit.
	finishCanceled()
	<-old.Done
	assert.NilError(t, old.Err)

	c.Lock()
	pid, actualExitCode, next := c.TaskCleanupPID, c.State.ExitCode, c.TaskCleanup
	c.Unlock()
	assert.Equal(t, pid, newPID)
	assert.Assert(t, next != nil)
	assert.Equal(t, actualExitCode, int(exitCode))
	assert.Check(t, !c.State.IsRunning())
	cleaner.results <- context.DeadlineExceeded
	<-next.Done
}
