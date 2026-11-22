package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/container"
)

const taskDeleteTimeout = 30 * time.Second

type exitedTaskCleaner interface {
	CleanupExitedTask(ctx context.Context, id string, pid uint32) error
}

// startTaskCleanup starts one recovery attempt, or returns the one in progress.
// The caller holds the container lock. The runtime call runs outside it.
func (daemon *Daemon) startTaskCleanup(c *container.Container) *container.TaskCleanupAttempt {
	if c.TaskCleanupPID == 0 || c.TaskCleanup != nil {
		return c.TaskCleanup
	}

	ctx, cancel := context.WithTimeout(daemon.taskCleanupContext, taskDeleteTimeout)
	attempt := &container.TaskCleanupAttempt{Done: make(chan struct{}), Cancel: cancel}
	c.TaskCleanup = attempt
	pid := c.TaskCleanupPID
	go func() {
		defer cancel()
		cleaner, ok := daemon.containerd.(exitedTaskCleaner)
		var err error
		if ok {
			err = cleaner.CleanupExitedTask(ctx, c.ID, pid)
		} else {
			err = errors.New("container runtime does not support exited task cleanup")
		}

		c.Lock()
		defer c.Unlock()
		defer close(attempt.Done)
		if c.TaskCleanup != attempt {
			// An in-flight delete can finish after cancellation. Leave the
			// replacement's state and resources alone.
			return
		}
		c.TaskCleanup = nil
		if err == nil {
			err = daemon.taskCleanupContext.Err()
		}
		if err != nil {
			attempt.Err = fmt.Errorf("could not finish deleting exited task for %s: %w", c.ID, err)
			log.G(ctx).Warn(attempt.Err)
			return
		}

		daemon.Cleanup(context.WithoutCancel(ctx), c)
		c.TaskCleanupPID = 0
		if err := c.CheckpointTo(context.WithoutCancel(ctx), daemon.containersReplica); err != nil {
			// Do not start a new task while the disk still records the
			// old task as pending cleanup.
			c.TaskCleanupPID = pid
			attempt.Err = fmt.Errorf("could not checkpoint task cleanup for %s: %w", c.ID, err)
			log.G(ctx).Warn(attempt.Err)
		}
	}()

	return attempt
}

// supersedeTaskCleanup invalidates recovery for a task that containerd replaced.
// The caller holds the container lock.
func supersedeTaskCleanup(c *container.Container) {
	c.TaskCleanupPID = 0
	if attempt := c.TaskCleanup; attempt != nil {
		c.TaskCleanup = nil
		attempt.Cancel()
	}
}

// awaitTaskCleanup is called with the container locked. It releases the lock
// while waiting, so callers must revalidate container state on return.
func (daemon *Daemon) awaitTaskCleanup(ctx context.Context, c *container.Container) error {
	for {
		attempt := daemon.startTaskCleanup(c)
		if attempt == nil {
			return nil
		}

		c.Unlock()
		var err error
		select {
		case <-attempt.Done:
			err = attempt.Err
		case <-ctx.Done():
			err = ctx.Err()
		}
		c.Lock()
		if err != nil {
			return err
		}
		// Another task may have exited before we reacquired the lock.
	}
}
