package exec

import (
	"fmt"
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/equality"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

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

// ControllerLogs defines a component that makes logs accessible.
//
// Can usually be accessed on a controller instance via type assertion.
type ControllerLogs interface {
	// Logs will write publisher until the context is cancelled or an error
	// occurs.
	Logs(ctx context.Context, publisher LogPublisher, options api.LogSubscriptionOptions) error
}

// LogPublisher defines the protocol for receiving a log message.
type LogPublisher interface {
	Publish(ctx context.Context, message api.LogMessage) error
}

// LogPublisherFunc implements publisher with just a function.
type LogPublisherFunc func(ctx context.Context, message api.LogMessage) error

// Publish calls the wrapped function.
func (fn LogPublisherFunc) Publish(ctx context.Context, message api.LogMessage) error {
	return fn(ctx, message)
}

// LogPublisherProvider defines the protocol for receiving a log publisher
type LogPublisherProvider interface {
	Publisher(ctx context.Context, subscriptionID string) (LogPublisher, func(), error)
}

// ContainerStatuser reports status of a container.
//
// This can be implemented by controllers or error types.
type ContainerStatuser interface {
	// ContainerStatus returns the status of the target container, if
	// available. When the container is not available, the status will be nil.
	ContainerStatus(ctx context.Context) (*api.ContainerStatus, error)
}

// PortStatuser reports status of ports which are allocated by the executor
type PortStatuser interface {
	// PortStatus returns the status on a list of PortConfigs
	// which are managed at the host level by the controller.
	PortStatus(ctx context.Context) (*api.PortStatus, error)
}

// Resolve attempts to get a controller from the executor and reports the
// correct status depending on the tasks current state according to the result.
//
// Unlike Do, if an error is returned, the status should still be reported. The
// error merely reports the failure at getting the controller.
func Resolve(ctx context.Context, task *api.Task, executor Executor) (Controller, *api.TaskStatus, error) {
	status := task.Status.Copy()

	defer func() {
		logStateChange(ctx, task.DesiredState, task.Status.State, status.State)
	}()

	ctlr, err := executor.Controller(task)

	// depending on the tasks state, a failed controller resolution has varying
	// impact. The following expresses that impact.
	if err != nil {
		status.Message = "resolving controller failed"
		status.Err = err.Error()
		// before the task has been started, we consider it a rejection.
		// if task is running, consider the task has failed
		// otherwise keep the existing state
		if task.Status.State < api.TaskStateStarting {
			status.State = api.TaskStateRejected
		} else if task.Status.State <= api.TaskStateRunning {
			status.State = api.TaskStateFailed
		}
	} else if task.Status.State < api.TaskStateAccepted {
		// we always want to proceed to accepted when we resolve the controller
		status.Message = "accepted"
		status.State = api.TaskStateAccepted
	}

	return ctlr, status, err
}

// Do progresses the task state using the controller performing a single
// operation on the controller. The return TaskStatus should be marked as the
// new state of the task.
//
// The returned status should be reported and placed back on to task
// before the next call. The operation can be cancelled by creating a
// cancelling context.
//
// Errors from the task controller will reported on the returned status. Any
// errors coming from this function should not be reported as related to the
// individual task.
//
// If ErrTaskNoop is returned, it means a second call to Do will result in no
// change. If ErrTaskDead is returned, calls to Do will no longer result in any
// action.
func Do(ctx context.Context, task *api.Task, ctlr Controller) (*api.TaskStatus, error) {
	status := task.Status.Copy()

	// stay in the current state.
	noop := func(errs ...error) (*api.TaskStatus, error) {
		return status, ErrTaskNoop
	}

	retry := func() (*api.TaskStatus, error) {
		// while we retry on all errors, this allows us to explicitly declare
		// retry cases.
		return status, ErrTaskRetry
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

	// containerStatus exitCode keeps track of whether or not we've set it in
	// this particular method. Eventually, we assemble this as part of a defer.
	var (
		containerStatus *api.ContainerStatus
		portStatus      *api.PortStatus
		exitCode        int
	)

	// returned when a fatal execution of the task is fatal. In this case, we
	// proceed to a terminal error state and set the appropriate fields.
	//
	// Common checks for the nature of an error should be included here. If the
	// error is determined not to be fatal for the task,
	fatal := func(err error) (*api.TaskStatus, error) {
		if err == nil {
			panic("err must not be nil when fatal")
		}

		if cs, ok := err.(ContainerStatuser); ok {
			var err error
			containerStatus, err = cs.ContainerStatus(ctx)
			if err != nil && !contextDoneError(err) {
				log.G(ctx).WithError(err).Error("error resolving container status on fatal")
			}
		}

		// make sure we've set the *correct* exit code
		if ec, ok := err.(ExitCoder); ok {
			exitCode = ec.ExitCode()
		}

		if cause := errors.Cause(err); cause == context.DeadlineExceeded || cause == context.Canceled {
			return retry()
		}

		status.Err = err.Error() // still reported on temporary
		if IsTemporary(err) {
			return retry()
		}

		// only at this point do we consider the error fatal to the task.
		log.G(ctx).WithError(err).Error("fatal task error")

		// NOTE(stevvooe): The following switch dictates the terminal failure
		// state based on the state in which the failure was encountered.
		switch {
		case status.State < api.TaskStateStarting:
			status.State = api.TaskStateRejected
		case status.State >= api.TaskStateStarting:
			status.State = api.TaskStateFailed
		}

		return status, nil
	}

	// below, we have several callbacks that are run after the state transition
	// is completed.
	defer func() {
		logStateChange(ctx, task.DesiredState, task.Status.State, status.State)

		if !equality.TaskStatusesEqualStable(status, &task.Status) {
			status.Timestamp = ptypes.MustTimestampProto(time.Now())
		}
	}()

	// extract the container status from the container, if supported.
	defer func() {
		// only do this if in an active state
		if status.State < api.TaskStateStarting {
			return
		}

		if containerStatus == nil {
			// collect this, if we haven't
			cctlr, ok := ctlr.(ContainerStatuser)
			if !ok {
				return
			}

			var err error
			containerStatus, err = cctlr.ContainerStatus(ctx)
			if err != nil && !contextDoneError(err) {
				log.G(ctx).WithError(err).Error("container status unavailable")
			}

			// at this point, things have gone fairly wrong. Remain positive
			// and let's get something out the door.
			if containerStatus == nil {
				containerStatus = new(api.ContainerStatus)
				containerStatusTask := task.Status.GetContainer()
				if containerStatusTask != nil {
					*containerStatus = *containerStatusTask // copy it over.
				}
			}
		}

		// at this point, we *must* have a containerStatus.
		if exitCode != 0 {
			containerStatus.ExitCode = int32(exitCode)
		}

		status.RuntimeStatus = &api.TaskStatus_Container{
			Container: containerStatus,
		}

		if portStatus == nil {
			pctlr, ok := ctlr.(PortStatuser)
			if !ok {
				return
			}

			var err error
			portStatus, err = pctlr.PortStatus(ctx)
			if err != nil && !contextDoneError(err) {
				log.G(ctx).WithError(err).Error("container port status unavailable")
			}
		}

		status.PortStatus = portStatus
	}()

	if task.DesiredState == api.TaskStateShutdown {
		if status.State >= api.TaskStateCompleted {
			return noop()
		}

		if err := ctlr.Shutdown(ctx); err != nil {
			return fatal(err)
		}

		return transition(api.TaskStateShutdown, "shutdown")
	}

	if status.State > task.DesiredState {
		return noop() // way beyond desired state, pause
	}

	// the following states may proceed past desired state.
	switch status.State {
	case api.TaskStatePreparing:
		if err := ctlr.Prepare(ctx); err != nil && err != ErrTaskPrepared {
			return fatal(err)
		}

		return transition(api.TaskStateReady, "prepared")
	case api.TaskStateStarting:
		if err := ctlr.Start(ctx); err != nil && err != ErrTaskStarted {
			return fatal(err)
		}

		return transition(api.TaskStateRunning, "started")
	case api.TaskStateRunning:
		if err := ctlr.Wait(ctx); err != nil {
			return fatal(err)
		}

		return transition(api.TaskStateCompleted, "finished")
	}

	// The following represent "pause" states. We can only proceed when the
	// desired state is beyond our current state.
	if status.State >= task.DesiredState {
		return noop()
	}

	switch status.State {
	case api.TaskStateNew, api.TaskStatePending, api.TaskStateAssigned:
		return transition(api.TaskStateAccepted, "accepted")
	case api.TaskStateAccepted:
		return transition(api.TaskStatePreparing, "preparing")
	case api.TaskStateReady:
		return transition(api.TaskStateStarting, "starting")
	default: // terminal states
		return noop()
	}
}

func logStateChange(ctx context.Context, desired, previous, next api.TaskState) {
	if previous != next {
		fields := logrus.Fields{
			"state.transition": fmt.Sprintf("%v->%v", previous, next),
			"state.desired":    desired,
		}
		log.G(ctx).WithFields(fields).Debug("state changed")
	}
}

func contextDoneError(err error) bool {
	cause := errors.Cause(err)
	return cause == context.Canceled || cause == context.DeadlineExceeded
}
