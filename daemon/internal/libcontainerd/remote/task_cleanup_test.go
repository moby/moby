package remote

import (
	"context"
	"errors"
	"testing"

	apitasks "github.com/containerd/containerd/api/services/tasks/v1"
	containerd "github.com/containerd/containerd/v2/client"
	cerrdefs "github.com/containerd/errdefs"
	libcontainerdtypes "github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gotest.tools/v3/assert"
)

type cleanupContainer struct {
	libcontainerdtypes.Container
	task libcontainerdtypes.Task
	err  error
}

func (c cleanupContainer) Task(context.Context) (libcontainerdtypes.Task, error) {
	return c.task, c.err
}

type cleanupTask struct {
	libcontainerdtypes.Task
	pid     uint32
	deleted bool
	err     error
}

func (task *cleanupTask) Pid() uint32 { return task.pid }

func (task *cleanupTask) Delete(context.Context) (*containerd.ExitStatus, error) {
	task.deleted = true
	return nil, task.err
}

type cleanupTaskService struct {
	apitasks.TasksClient
	deleted string
	err     error
}

func (s *cleanupTaskService) Delete(_ context.Context, req *apitasks.DeleteTaskRequest, _ ...grpc.CallOption) (*apitasks.DeleteResponse, error) {
	s.deleted = req.ContainerID
	return nil, s.err
}

func TestDeleteExitedTask(t *testing.T) {
	t.Run("delete the stopped task", func(t *testing.T) {
		task := &cleanupTask{pid: 123}
		svc := &cleanupTaskService{}
		err := deleteExitedTask(t.Context(), svc, cleanupContainer{task: task}, "container", 123)
		assert.NilError(t, err)
		assert.Check(t, task.deleted)
		assert.Equal(t, svc.deleted, "")
	})

	t.Run("finish deleting a stale shim", func(t *testing.T) {
		svc := &cleanupTaskService{err: status.Error(codes.NotFound, "task missing from shim")}
		err := deleteExitedTask(t.Context(), svc, cleanupContainer{err: cerrdefs.ErrNotFound}, "container", 123)
		assert.NilError(t, err)
		assert.Equal(t, svc.deleted, "container")
	})

	t.Run("finish deletion if task disappears after lookup", func(t *testing.T) {
		task := &cleanupTask{pid: 123, err: cerrdefs.ErrNotFound}
		svc := &cleanupTaskService{}
		err := deleteExitedTask(t.Context(), svc, cleanupContainer{task: task}, "container", 123)
		assert.NilError(t, err)
		assert.Check(t, task.deleted)
		assert.Equal(t, svc.deleted, "container")
	})

	t.Run("leave a different task alone", func(t *testing.T) {
		task := &cleanupTask{pid: 456}
		svc := &cleanupTaskService{}
		err := deleteExitedTask(t.Context(), svc, cleanupContainer{task: task}, "container", 123)
		assert.Check(t, cerrdefs.IsConflict(err))
		assert.Check(t, !task.deleted)
		assert.Equal(t, svc.deleted, "")
	})

	t.Run("skip direct delete after cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		svc := &cleanupTaskService{}
		err := deleteExitedTask(ctx, svc, cleanupContainer{err: cerrdefs.ErrNotFound}, "container", 123)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, svc.deleted, "")
	})

	t.Run("propagate a failed direct delete", func(t *testing.T) {
		svc := &cleanupTaskService{err: status.Error(codes.Unavailable, "shim unavailable")}
		err := deleteExitedTask(t.Context(), svc, cleanupContainer{err: cerrdefs.ErrNotFound}, "container", 123)
		assert.Check(t, errors.Is(err, cerrdefs.ErrUnavailable))
		assert.Equal(t, svc.deleted, "container")
	})
}
