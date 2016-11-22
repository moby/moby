package agent

import (
	"time"

	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/equality"
	"github.com/docker/swarmkit/log"
	"golang.org/x/net/context"
)

// taskManager manages all aspects of task execution and reporting for an agent
// through state management.
type taskManager struct {
	task     *api.Task
	ctlr     exec.Controller
	reporter StatusReporter

	updateq chan *api.Task

	shutdown chan struct{}
	closed   chan struct{}
}

func newTaskManager(ctx context.Context, task *api.Task, ctlr exec.Controller, reporter StatusReporter) *taskManager {
	t := &taskManager{
		task:     task.Copy(),
		ctlr:     ctlr,
		reporter: reporter,
		updateq:  make(chan *api.Task),
		shutdown: make(chan struct{}),
		closed:   make(chan struct{}),
	}
	go t.run(ctx)
	return t
}

// Update the task data.
func (tm *taskManager) Update(ctx context.Context, task *api.Task) error {
	select {
	case tm.updateq <- task:
		return nil
	case <-tm.closed:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close shuts down the task manager, blocking until it is stopped.
func (tm *taskManager) Close() error {
	select {
	case <-tm.closed:
		return nil
	case <-tm.shutdown:
	default:
		close(tm.shutdown)
	}

	select {
	case <-tm.closed:
		return nil
	}
}

func (tm *taskManager) Logs(ctx context.Context, options api.LogSubscriptionOptions, publisher exec.LogPublisher) {
	ctx = log.WithModule(ctx, "taskmanager")

	logCtlr, ok := tm.ctlr.(exec.ControllerLogs)
	if !ok {
		return // no logs available
	}
	if err := logCtlr.Logs(ctx, publisher, options); err != nil {
		log.G(ctx).WithError(err).Errorf("logs call failed")
	}
}

func (tm *taskManager) run(ctx context.Context) {
	ctx, cancelAll := context.WithCancel(ctx)
	defer cancelAll() // cancel all child operations on exit.

	ctx = log.WithModule(ctx, "taskmanager")

	var (
		opctx    context.Context
		cancel   context.CancelFunc
		run      = make(chan struct{}, 1)
		statusq  = make(chan *api.TaskStatus)
		errs     = make(chan error)
		shutdown = tm.shutdown
		updated  bool // true if the task was updated.
	)

	defer func() {
		// closure  picks up current value of cancel.
		if cancel != nil {
			cancel()
		}
	}()

	run <- struct{}{} // prime the pump
	for {
		select {
		case <-run:
			// always check for shutdown before running.
			select {
			case <-tm.shutdown:
				continue // ignore run request and handle shutdown
			case <-tm.closed:
				continue
			default:
			}

			opctx, cancel = context.WithCancel(ctx)

			// Several variables need to be snapshotted for the closure below.
			opcancel := cancel        // fork for the closure
			running := tm.task.Copy() // clone the task before dispatch
			statusqLocal := statusq
			updatedLocal := updated // capture state of update for goroutine
			updated = false
			go runctx(ctx, tm.closed, errs, func(ctx context.Context) error {
				defer opcancel()

				if updatedLocal {
					// before we do anything, update the task for the controller.
					// always update the controller before running.
					if err := tm.ctlr.Update(opctx, running); err != nil {
						log.G(ctx).WithError(err).Error("updating task controller failed")
						return err
					}
				}

				status, err := exec.Do(opctx, running, tm.ctlr)
				if status != nil {
					// always report the status if we get one back. This
					// returns to the manager loop, then reports the status
					// upstream.
					select {
					case statusqLocal <- status:
					case <-ctx.Done(): // not opctx, since that may have been cancelled.
					}

					if err := tm.reporter.UpdateTaskStatus(ctx, running.ID, status); err != nil {
						log.G(ctx).WithError(err).Error("failed reporting status to agent")
					}
				}

				return err
			})
		case err := <-errs:
			// This branch is always executed when an operations completes. The
			// goal is to decide whether or not we re-dispatch the operation.
			cancel = nil

			select {
			case <-tm.shutdown:
				shutdown = tm.shutdown // re-enable the shutdown branch
				continue               // no dispatch if we are in shutdown.
			default:
			}

			switch err {
			case exec.ErrTaskNoop:
				if !updated {
					continue // wait till getting pumped via update.
				}
			case exec.ErrTaskRetry:
				// TODO(stevvooe): Add exponential backoff with random jitter
				// here. For now, this backoff is enough to keep the task
				// manager from running away with the CPU.
				time.AfterFunc(time.Second, func() {
					errs <- nil // repump this branch, with no err
				})
				continue
			case nil, context.Canceled, context.DeadlineExceeded:
				// no log in this case
			default:
				log.G(ctx).WithError(err).Error("task operation failed")
			}

			select {
			case run <- struct{}{}:
			default:
			}
		case status := <-statusq:
			tm.task.Status = *status
		case task := <-tm.updateq:
			if equality.TasksEqualStable(task, tm.task) {
				continue // ignore the update
			}

			if task.ID != tm.task.ID {
				log.G(ctx).WithField("task.update.id", task.ID).Error("received update for incorrect task")
				continue
			}

			if task.DesiredState < tm.task.DesiredState {
				log.G(ctx).WithField("task.update.desiredstate", task.DesiredState).
					Error("ignoring task update with invalid desired state")
				continue
			}

			task = task.Copy()
			task.Status = tm.task.Status // overwrite our status, as it is canonical.
			tm.task = task
			updated = true

			// we have accepted the task update
			if cancel != nil {
				cancel() // cancel outstanding if necessary.
			} else {
				// If this channel op fails, it means there is already a
				// message on the run queue.
				select {
				case run <- struct{}{}:
				default:
				}
			}
		case <-shutdown:
			if cancel != nil {
				// cancel outstanding operation.
				cancel()

				// subtle: after a cancellation, we want to avoid busy wait
				// here. this gets renabled in the errs branch and we'll come
				// back around and try shutdown again.
				shutdown = nil // turn off this branch until op proceeds
				continue       // wait until operation actually exits.
			}

			// TODO(stevvooe): This should be left for the repear.

			// make an attempt at removing. this is best effort. any errors will be
			// retried by the reaper later.
			if err := tm.ctlr.Remove(ctx); err != nil {
				log.G(ctx).WithError(err).WithField("task.id", tm.task.ID).Error("remove task failed")
			}

			if err := tm.ctlr.Close(); err != nil {
				log.G(ctx).WithError(err).Error("error closing controller")
			}
			// disable everything, and prepare for closing.
			statusq = nil
			errs = nil
			shutdown = nil
			close(tm.closed)
		case <-tm.closed:
			return
		case <-ctx.Done():
			return
		}
	}
}
