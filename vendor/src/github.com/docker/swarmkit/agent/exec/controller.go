package exec

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"golang.org/x/net/context"
)

// ContainerController controls execution of container tasks.
type ContainerController interface {
	// ContainerStatus returns the status of the target container, if
	// available. When the container is not available, the status will be nil.
	ContainerStatus(ctx context.Context) (*api.ContainerStatus, error)
}

// Controller controls execution of a task.
type Controller interface {
	// Update the task definition seen by the controller. Will return
	// ErrTaskUpdateFailed if the provided task definition changes fields that
	// cannot be changed.
	//
	// Will be ignored if the task has exited.
	Update(ctx context.Context, t *api.Task) error

	// Prepare the task for execution. This should ensure that all resources
	// are created such that a call to start should execute immediately.
	Prepare(ctx context.Context) error

	// Start the target and return when it has started successfully.
	Start(ctx context.Context) error

	// Wait blocks until the target has exited.
	Wait(ctx context.Context) error

	// Shutdown requests to exit the target gracefully.
	Shutdown(ctx context.Context) error

	// Terminate the target.
	Terminate(ctx context.Context) error

	// Remove all resources allocated by the controller.
	Remove(ctx context.Context) error

	// Close closes any ephemeral resources associated with controller instance.
	Close() error
}

// Reporter defines an interface for calling back into the task status
// reporting infrastructure. Typically, an instance is associated to a specific
// task.
//
// The results of the "Report" are combined with a TaskStatus and sent to the
// dispatcher.
type Reporter interface {
	// Report the state of the task run. If an error is returned, execution
	// will be stopped.
	// TODO(aluzzardi): This interface leaks ContainerStatus and needs fixing.
	Report(ctx context.Context, state api.TaskState, msg string, cstatus *api.ContainerStatus) error

	// TODO(stevvooe): It is very likely we will need to report more
	// information back from the controller into the agent. We'll likely expand
	// this interface to do so.
}

// Run runs a controller, reporting state along the way. Under normal execution,
// this function blocks until the task is completed.
func Run(ctx context.Context, ctlr Controller, reporter Reporter) error {
	if err := report(ctx, reporter, api.TaskStatePreparing, "preparing", nil); err != nil {
		return err
	}

	if err := ctlr.Prepare(ctx); err != nil {
		switch err {
		case ErrTaskPrepared:
			log.G(ctx).Warnf("already prepared")
			return runStart(ctx, ctlr, reporter, "already prepared")
		case ErrTaskStarted:
			log.G(ctx).Warnf("already started")
			return runWait(ctx, ctlr, reporter, "already started")
		default:
			return err
		}
	}

	if err := report(ctx, reporter, api.TaskStateReady, "prepared", nil); err != nil {
		return err
	}

	return runStart(ctx, ctlr, reporter, "starting")
}

// Shutdown the task using the controller and report on the status.
func Shutdown(ctx context.Context, ctlr Controller, reporter Reporter) error {
	if err := ctlr.Shutdown(ctx); err != nil {
		return err
	}

	return report(ctx, reporter, api.TaskStateShutdown, "shutdown requested", nil)
}

// runStart reports that the task is starting, calls Start and hands execution
// off to `runWait`. It will block until task execution is completed or an
// error is encountered.
func runStart(ctx context.Context, ctlr Controller, reporter Reporter, msg string) error {
	if err := report(ctx, reporter, api.TaskStateStarting, msg, nil); err != nil {
		return err
	}

	msg = "started"
	if err := ctlr.Start(ctx); err != nil {
		switch err {
		case ErrTaskStarted:
			log.G(ctx).Warnf("already started")
			msg = "already started"
		default:
			return err
		}
	}

	return runWait(ctx, ctlr, reporter, msg)
}

// runWait reports that the task is running and calls Wait. When Wait exits,
// the task will be reported as completed.
func runWait(ctx context.Context, ctlr Controller, reporter Reporter, msg string) error {
	getContainerStatus := func() (*api.ContainerStatus, error) {
		if cs, ok := ctlr.(ContainerController); ok {
			return cs.ContainerStatus(ctx)
		}
		return nil, nil
	}

	cstatus, err := getContainerStatus()
	if err != nil {
		return err
	}

	if err := report(ctx, reporter, api.TaskStateRunning, msg, cstatus); err != nil {
		return err
	}

	if err := ctlr.Wait(ctx); err != nil {
		// NOTE(stevvooe): We *do not* handle the exit error here,
		// since we may do something different based on whether we
		// are in SHUTDOWN or having an unplanned exit,
		return err
	}

	cstatus, err = getContainerStatus()
	if err != nil {
		return err
	}

	return report(ctx, reporter, api.TaskStateCompleted, "completed", cstatus)
}

func report(ctx context.Context, reporter Reporter, state api.TaskState, msg string, cstatus *api.ContainerStatus) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(
		logrus.Fields{
			"state":      state,
			"status.msg": msg}))
	log.G(ctx).Debug("report status")
	return reporter.Report(ctx, state, msg, cstatus)
}

// Do progresses the task state using the controller by a single operation
// on the controller. The return TaskStatus should be marked as the new state
// of the task.
//
// The returned status should be reported and placed back on to task
// before the next call. The operation can be cancelled by creating a
// cancelling context.
//
// Errors from the task controller will reported on the returned status. Any
// errors coming from this function should not be reported as related to the
// individual task.
func Do(ctx context.Context, task *api.Task, ctlr Controller) (*api.TaskStatus, error) {
	status := task.Status.Copy()

	// stay in the current state.
	noop := func(errs ...error) (*api.TaskStatus, error) {
		// TODO(stevvooe): May want to return sentinal error here to
		// communicate that we cannot proceed past the current state.
		return status, nil
	}

	// transition moves the task to the next state.
	transition := func(state api.TaskState, msg string) (*api.TaskStatus, error) {
		current := status.State
		status.State = state
		status.Message = msg

		if current > state {
			panic("invalid state transition")
		}
		return status, nil
	}

	// returned when a fatal execution of the task is fatal. In this case, we
	// proceed to a terminal error state and set the appropriate fields.
	//
	// Common checks for the nature of an error should be included here. If the
	// error is determined not to be fatal for the task,
	fatal := func(err error) (*api.TaskStatus, error) {
		log.G(ctx).WithError(err).Error("fatal task error")
		if err == nil {
			panic("err must not be nil when fatal")
		}

		if err, ok := err.(Temporary); ok && err.Temporary() {
			return noop()
		}

		if err == context.DeadlineExceeded || err == context.Canceled {
			return noop()
		}

		status.Err = err.Error()

		switch {
		case status.State < api.TaskStateStarting:
			status.State = api.TaskStateRejected
		case status.State > api.TaskStateStarting:
			status.State = api.TaskStateFailed
		}

		return status, nil
	}

	// below, we have several callbacks that are run after the state transition
	// is completed.

	defer func() {
		log.G(ctx).WithField("state.transition", fmt.Sprintf("%v->%v", task.Status.State, status.State)).
			Info("state changed")
	}()

	// extract the container status from the container, if supported.
	defer func() {
		// only do this if in an active state
		cctlr, ok := ctlr.(ContainerController)
		if !ok {
			return
		}

		cstatus, err := cctlr.ContainerStatus(ctx)
		if err != nil {
			log.G(ctx).WithError(err).Error("container status unavailable")
			return
		}

		if cstatus != nil {
			status.RuntimeStatus = &api.TaskStatus_Container{
				Container: cstatus,
			}
		}
	}()

	switch task.DesiredState {
	case api.TaskStateNew, api.TaskStateAllocated,
		api.TaskStateAssigned, api.TaskStateAccepted,
		api.TaskStatePreparing, api.TaskStateReady,
		api.TaskStateStarting, api.TaskStateRunning,
		api.TaskStateCompleted, api.TaskStateFailed,
		api.TaskStateRejected:

		if task.DesiredState < status.State {
			// do not yet proceed. the desired state is less than the current
			// state.
			return noop()
		}

		switch status.State {
		case api.TaskStateNew, api.TaskStateAllocated,
			api.TaskStateAssigned:
			return transition(api.TaskStateAccepted, "accepted")
		case api.TaskStateAccepted:
			return transition(api.TaskStatePreparing, "preparing")
		case api.TaskStatePreparing:
			if err := ctlr.Prepare(ctx); err != nil && err != ErrTaskPrepared {
				return fatal(err)
			}

			return transition(api.TaskStateReady, "prepared")
		case api.TaskStateReady:
			return transition(api.TaskStateStarting, "starting")
		case api.TaskStateStarting:
			if err := ctlr.Start(ctx); err != nil && err != ErrTaskStarted {
				return fatal(err)
			}

			return transition(api.TaskStateRunning, "started")
		case api.TaskStateRunning:
			if err := ctlr.Wait(ctx); err != nil {
				if _, ok := err.(*ExitError); ok {
					return transition(api.TaskStateFailed, "failed")
				}

				// TODO(stevvooe): In most cases, failures of Wait are actually
				// not a failure of the task. We account for this in fatal by
				// checking temporary.
				return fatal(err)
			}

			return transition(api.TaskStateCompleted, "finished")
		}
	case api.TaskStateShutdown:
		if status.State >= api.TaskStateShutdown {
			return noop()
		}

		if err := ctlr.Shutdown(ctx); err != nil {
			return fatal(err)
		}

		return transition(api.TaskStateShutdown, "shutdown")
	}

	panic("not reachable")
}
