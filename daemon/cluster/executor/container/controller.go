package container

import (
	"errors"
	"strings"

	executorpkg "github.com/docker/docker/daemon/cluster/executor"
	"github.com/docker/engine-api/types"
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"golang.org/x/net/context"
)

// controller implements agent.Controller against docker's API.
//
// Most operations against docker's API are done through the container name,
// which is unique to the task.
type controller struct {
	backend executorpkg.Backend
	task    *api.Task
	adapter *containerAdapter
	closed  chan struct{}
	err     error
}

var _ exec.Controller = &controller{}

// NewController returns a dockerexec runner for the provided task.
func newController(b executorpkg.Backend, task *api.Task) (*controller, error) {
	adapter, err := newContainerAdapter(b, task)
	if err != nil {
		return nil, err
	}

	return &controller{
		backend: b,
		task:    task,
		adapter: adapter,
		closed:  make(chan struct{}),
	}, nil
}

func (r *controller) Task() (*api.Task, error) {
	return r.task, nil
}

// ContainerStatus returns the container-specific status for the task.
func (r *controller) ContainerStatus(ctx context.Context) (*api.ContainerStatus, error) {
	ctnr, err := r.adapter.inspect(ctx)
	if err != nil {
		if isUnknownContainer(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseContainerStatus(ctnr)
}

// Update tasks a recent task update and applies it to the container.
func (r *controller) Update(ctx context.Context, t *api.Task) error {
	log.G(ctx).Warnf("task updates not yet supported")
	// TODO(stevvooe): While assignment of tasks is idempotent, we do allow
	// updates of metadata, such as labelling, as well as any other properties
	// that make sense.
	return nil
}

// Prepare creates a container and ensures the image is pulled.
//
// If the container has already be created, exec.ErrTaskPrepared is returned.
func (r *controller) Prepare(ctx context.Context) error {
	if err := r.checkClosed(); err != nil {
		return err
	}

	// Make sure all the networks that the task needs are created.
	if err := r.adapter.createNetworks(ctx); err != nil {
		return err
	}

	// Make sure all the volumes that the task needs are created.
	if err := r.adapter.createVolumes(ctx, r.backend); err != nil {
		return err
	}

	for {
		if err := r.checkClosed(); err != nil {
			return err
		}
		if err := r.adapter.create(ctx, r.backend); err != nil {
			if isContainerCreateNameConflict(err) {
				if _, err := r.adapter.inspect(ctx); err != nil {
					return err
				}

				// container is already created. success!
				return exec.ErrTaskPrepared
			}

			if !strings.Contains(err.Error(), "No such image") { // todo: better error detection
				return err
			}
			if err := r.adapter.pullImage(ctx); err != nil {
				return err
			}

			continue // retry to create the container
		}

		break
	}

	return nil
}

// Start the container. An error will be returned if the container is already started.
func (r *controller) Start(ctx context.Context) error {
	if err := r.checkClosed(); err != nil {
		return err
	}

	ctnr, err := r.adapter.inspect(ctx)
	if err != nil {
		return err
	}

	// Detect whether the container has *ever* been started. If so, we don't
	// issue the start.
	//
	// TODO(stevvooe): This is very racy. While reading inspect, another could
	// start the process and we could end up starting it twice.
	if ctnr.State.Status != "created" {
		return exec.ErrTaskStarted
	}

	if err := r.adapter.start(ctx); err != nil {
		return err
	}

	return nil
}

// Wait on the container to exit.
func (r *controller) Wait(pctx context.Context) error {
	if err := r.checkClosed(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(pctx)
	defer cancel()

	c, err := r.adapter.wait(ctx)
	if err != nil {
		return err
	}

	// todo: event
	select {
	case <-c:
		ctnr, err := r.adapter.inspect(ctx)
		if err != nil {
			return err
		}

		if ctnr.State.ExitCode != 0 {
			var cause error
			if ctnr.State.Error != "" {
				cause = errors.New(ctnr.State.Error)
			}
			return &exec.ExitError{
				Code:  ctnr.State.ExitCode,
				Cause: cause,
			}
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// Shutdown the container cleanly.
func (r *controller) Shutdown(ctx context.Context) error {
	if err := r.checkClosed(); err != nil {
		return err
	}

	if err := r.adapter.shutdown(ctx); err != nil {
		if isUnknownContainer(err) {
			return nil
		}

		return err
	}

	return nil
}

// Terminate the container, with force.
func (r *controller) Terminate(ctx context.Context) error {
	if err := r.checkClosed(); err != nil {
		return err
	}

	if err := r.adapter.terminate(ctx); err != nil {
		if isUnknownContainer(err) {
			return nil
		}

		return err
	}

	return nil
}

// Remove the container and its resources.
func (r *controller) Remove(ctx context.Context) error {
	if err := r.checkClosed(); err != nil {
		return err
	}

	// It may be necessary to shut down the task before removing it.
	if err := r.Shutdown(ctx); err != nil {
		if isUnknownContainer(err) {
			return nil
		}
		// This may fail if the task was already shut down.
		log.G(ctx).WithError(err).Debug("shutdown failed on removal")
	}

	// Try removing networks referenced in this task in case this
	// task is the last one referencing it
	if err := r.adapter.removeNetworks(ctx); err != nil {
		if isUnknownContainer(err) {
			return nil
		}
		return err
	}

	if err := r.adapter.remove(ctx); err != nil {
		if isUnknownContainer(err) {
			return nil
		}

		return err
	}
	return nil
}

// Close the runner and clean up any ephemeral resources.
func (r *controller) Close() error {
	select {
	case <-r.closed:
		return r.err
	default:
		r.err = exec.ErrControllerClosed
		close(r.closed)
	}
	return nil
}

func (r *controller) checkClosed() error {
	select {
	case <-r.closed:
		return r.err
	default:
		return nil
	}
}

func parseContainerStatus(ctnr types.ContainerJSON) (*api.ContainerStatus, error) {
	status := &api.ContainerStatus{
		ContainerID: ctnr.ID,
		PID:         int32(ctnr.State.Pid),
		ExitCode:    int32(ctnr.State.ExitCode),
	}

	return status, nil
}
