package remote

import (
	"context"
	"errors"
	"time"

	apitasks "github.com/containerd/containerd/api/services/tasks/v1"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errgrpc"
	libcontainerdtypes "github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
	"github.com/moby/moby/v2/errdefs"
)

// CleanupExitedTask finishes deleting a task after an earlier delete timed out.
func (c *client) CleanupExitedTask(ctx context.Context, id string, pid uint32) error {
	ctr, err := c.LoadContainer(ctx, id)
	if cerrdefs.IsNotFound(err) {
		// The metadata may already have been removed by a recovery attempt
		// that finished deleting the task before the daemon stopped.
		return nil
	}
	if err != nil {
		return err
	}

	for {
		err = deleteExitedTask(ctx, c.client.TaskService(), ctr, id, pid)
		if !errors.Is(err, context.DeadlineExceeded) && !cerrdefs.IsDeadlineExceeded(err) {
			break
		}
		// Task status has its own deadline and can time out while the first
		// deletion still holds the shim's lock.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err = ctr.Delete(ctx)
	if cerrdefs.IsNotFound(err) {
		return nil
	}
	return err
}

func deleteExitedTask(ctx context.Context, service apitasks.TasksClient, ctr libcontainerdtypes.Container, id string, pid uint32) error {
	task, err := ctr.Task(ctx)
	if err == nil {
		if task.Pid() != pid {
			return errdefs.Conflict(errors.New("containerd task changed during exit cleanup"))
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err = task.Delete(ctx)
	}
	if !cerrdefs.IsNotFound(err) {
		return err
	}

	// Task lookup and Delete can report NotFound once the shim removed
	// the process, even while containerd still has the shim registered.
	// Finish the deletion through the task service in that case.
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err = service.Delete(ctx, &apitasks.DeleteTaskRequest{ContainerID: id})
	err = errgrpc.ToNative(err)
	if cerrdefs.IsNotFound(err) {
		return nil
	}
	return err
}
