package container

import (
	"fmt"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	executorpkg "github.com/docker/docker/daemon/cluster/executor"
	"github.com/docker/libnetwork"
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// controller implements agent.Controller against docker's API.
//
// Most operations against docker's API are done through the container name,
// which is unique to the task.
type controller struct {
	task    *api.Task
	adapter *containerAdapter
	closed  chan struct{}
	err     error

	pulled     chan struct{} // closed after pull
	cancelPull func()        // cancels pull context if not nil
	pullErr    error         // pull error, only read after pulled closed
}

var _ exec.Controller = &controller{}

// NewController returns a docker exec runner for the provided task.
func newController(b executorpkg.Backend, task *api.Task) (*controller, error) {
	adapter, err := newContainerAdapter(b, task)
	if err != nil {
		return nil, err
	}

	return &controller{
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
	if err := r.adapter.createVolumes(ctx); err != nil {
		return err
	}

	if os.Getenv("DOCKER_SERVICE_PREFER_OFFLINE_IMAGE") != "1" {
		if r.pulled == nil {
			// Fork the pull to a different context to allow pull to continue
			// on re-entrant calls to Prepare. This ensures that Prepare can be
			// idempotent and not incur the extra cost of pulling when
			// cancelled on updates.
			var pctx context.Context

			r.pulled = make(chan struct{})
			pctx, r.cancelPull = context.WithCancel(context.Background()) // TODO(stevvooe): Bind a context to the entire controller.

			go func() {
				defer close(r.pulled)
				r.pullErr = r.adapter.pullImage(pctx) // protected by closing r.pulled
			}()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.pulled:
			if r.pullErr != nil {
				// NOTE(stevvooe): We always try to pull the image to make sure we have
				// the most up to date version. This will return an error, but we only
				// log it. If the image truly doesn't exist, the create below will
				// error out.
				//
				// This gives us some nice behavior where we use up to date versions of
				// mutable tags, but will still run if the old image is available but a
				// registry is down.
				//
				// If you don't want this behavior, lock down your image to an
				// immutable tag or digest.
				log.G(ctx).WithError(r.pullErr).Error("pulling image failed")
			}
		}
	}

	if err := r.adapter.create(ctx); err != nil {
		if isContainerCreateNameConflict(err) {
			if _, err := r.adapter.inspect(ctx); err != nil {
				return err
			}

			// container is already created. success!
			return exec.ErrTaskPrepared
		}

		return err
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

	for {
		if err := r.adapter.start(ctx); err != nil {
			if _, ok := err.(libnetwork.ErrNoSuchNetwork); ok {
				// Retry network creation again if we
				// failed because some of the networks
				// were not found.
				if err := r.adapter.createNetworks(ctx); err != nil {
					return err
				}

				continue
			}

			return errors.Wrap(err, "starting container failed")
		}

		break
	}

	// no health check
	if ctnr.Config == nil || ctnr.Config.Healthcheck == nil {
		return nil
	}

	healthCmd := ctnr.Config.Healthcheck.Test

	if len(healthCmd) == 0 || healthCmd[0] == "NONE" {
		return nil
	}

	// wait for container to be healthy
	eventq := r.adapter.events(ctx)

	var healthErr error
	for {
		select {
		case event := <-eventq:
			if !r.matchevent(event) {
				continue
			}

			switch event.Action {
			case "die": // exit on terminal events
				ctnr, err := r.adapter.inspect(ctx)
				if err != nil {
					return errors.Wrap(err, "die event received")
				} else if ctnr.State.ExitCode != 0 {
					return &exitError{code: ctnr.State.ExitCode, cause: healthErr}
				}

				return nil
			case "destroy":
				// If we get here, something has gone wrong but we want to exit
				// and report anyways.
				return ErrContainerDestroyed
			case "health_status: unhealthy":
				// in this case, we stop the container and report unhealthy status
				if err := r.Shutdown(ctx); err != nil {
					return errors.Wrap(err, "unhealthy container shutdown failed")
				}
				// set health check error, and wait for container to fully exit ("die" event)
				healthErr = ErrContainerUnhealthy
			case "health_status: healthy":
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-r.closed:
			return r.err
		}
	}
}

// Wait on the container to exit.
func (r *controller) Wait(pctx context.Context) error {
	if err := r.checkClosed(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(pctx)
	defer cancel()

	healthErr := make(chan error, 1)
	go func() {
		ectx, cancel := context.WithCancel(ctx) // cancel event context on first event
		defer cancel()
		if err := r.checkHealth(ectx); err == ErrContainerUnhealthy {
			healthErr <- ErrContainerUnhealthy
			if err := r.Shutdown(ectx); err != nil {
				log.G(ectx).WithError(err).Debug("shutdown failed on unhealthy")
			}
		}
	}()

	err := r.adapter.wait(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err != nil {
		ee := &exitError{}
		if ec, ok := err.(exec.ExitCoder); ok {
			ee.code = ec.ExitCode()
		}
		select {
		case e := <-healthErr:
			ee.cause = e
		default:
			if err.Error() != "" {
				ee.cause = err
			}
		}
		return ee
	}

	return nil
}

// Shutdown the container cleanly.
func (r *controller) Shutdown(ctx context.Context) error {
	if err := r.checkClosed(); err != nil {
		return err
	}

	if r.cancelPull != nil {
		r.cancelPull()
	}

	if err := r.adapter.shutdown(ctx); err != nil {
		if isUnknownContainer(err) || isStoppedContainer(err) {
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

	if r.cancelPull != nil {
		r.cancelPull()
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

	if r.cancelPull != nil {
		r.cancelPull()
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
		if r.cancelPull != nil {
			r.cancelPull()
		}

		r.err = exec.ErrControllerClosed
		close(r.closed)
	}
	return nil
}

func (r *controller) matchevent(event events.Message) bool {
	if event.Type != events.ContainerEventType {
		return false
	}

	// TODO(stevvooe): Filter based on ID matching, in addition to name.

	// Make sure the events are for this container.
	if event.Actor.Attributes["name"] != r.adapter.container.name() {
		return false
	}

	return true
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

type exitError struct {
	code  int
	cause error
}

func (e *exitError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("task: non-zero exit (%v): %v", e.code, e.cause)
	}

	return fmt.Sprintf("task: non-zero exit (%v)", e.code)
}

func (e *exitError) ExitCode() int {
	return int(e.code)
}

func (e *exitError) Cause() error {
	return e.cause
}

// checkHealth blocks until unhealthy container is detected or ctx exits
func (r *controller) checkHealth(ctx context.Context) error {
	eventq := r.adapter.events(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.closed:
			return nil
		case event := <-eventq:
			if !r.matchevent(event) {
				continue
			}

			switch event.Action {
			case "health_status: unhealthy":
				return ErrContainerUnhealthy
			}
		}
	}
}
