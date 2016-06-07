package agent

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"golang.org/x/net/context"
)

const (
	initialSessionFailureBackoff = time.Second
	maxSessionFailureBackoff     = 8 * time.Second
)

// Agent implements the primary node functionality for a member of a swarm
// cluster. The primary functionality id to run and report on the status of
// tasks assigned to the node.
type Agent struct {
	config *Config

	// The latest node object state from manager
	// for this node known to the agent.
	node *api.Node

	tasks       map[string]*api.Task        // contains all managed tasks
	assigned    map[string]*api.Task        // contains current assignment set
	controllers map[string]exec.Controller  // contains all controllers
	reports     map[string]taskStatusReport // pending reports, indexed by task ID
	shutdown    map[string]struct{}         // control shutdown jobs
	remove      map[string]struct{}         // control shutdown jobs
	keys        []*api.EncryptionKey

	statusq  chan taskStatusReport
	removedq chan string

	started chan struct{}
	ready   chan struct{}
	stopped chan struct{} // requests shutdown
	closed  chan struct{} // only closed in run
	err     error         // read only after closed is closed
	mu      sync.Mutex
}

// New returns a new agent, ready for task dispatch.
func New(config *Config) (*Agent, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	ag := &Agent{
		config:      config,
		tasks:       make(map[string]*api.Task),
		assigned:    make(map[string]*api.Task),
		controllers: make(map[string]exec.Controller),
		reports:     make(map[string]taskStatusReport),
		shutdown:    make(map[string]struct{}),
		remove:      make(map[string]struct{}),

		statusq:  make(chan taskStatusReport),
		removedq: make(chan string),

		started: make(chan struct{}),
		ready:   make(chan struct{}),
		stopped: make(chan struct{}),
		closed:  make(chan struct{}),
	}
	return ag, nil
}

var (
	errAgentNotStarted = errors.New("agent: not started")
	errAgentStarted    = errors.New("agent: already started")
	errAgentStopped    = errors.New("agent: stopped")

	errTaskNoContoller            = errors.New("agent: no task controller")
	errTaskNotAssigned            = errors.New("agent: task not assigned")
	errTaskInvalidStateTransition = errors.New("agent: invalid task transition")
	errTaskStatusUpdateNoChange   = errors.New("agent: no change in task status")
	errTaskDead                   = errors.New("agent: task dead")
	errTaskUnknown                = errors.New("agent: task unknown")
)

// Start begins execution of the agent in the provided context, if not already
// started.
func (a *Agent) Start(ctx context.Context) error {
	select {
	case <-a.started:
		select {
		case <-a.closed:
			return a.err
		case <-a.stopped:
			return errAgentStopped
		case <-ctx.Done():
			return ctx.Err()
		default:
			return errAgentStarted
		}
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	close(a.started)
	go a.run(ctx)

	return nil
}

// Stop shuts down the agent, blocking until full shutdown. If the agent is not
// started, Stop will block until Started.
func (a *Agent) Stop(ctx context.Context) error {
	select {
	case <-a.started:
		select {
		case <-a.closed:
			return a.err
		case <-a.stopped:
			select {
			case <-a.closed:
				return a.err
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return ctx.Err()
		default:
			close(a.stopped)
			// recurse and wait for closure
			return a.Stop(ctx)
		}
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errAgentNotStarted
	}
}

// Err returns the error that caused the agent to shutdown or nil. Err blocks
// until the agent is fully shutdown.
func (a *Agent) Err(ctx context.Context) error {
	select {
	case <-a.closed:
		return a.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Ready returns a channel that will be closed when agent first becomes ready.
func (a *Agent) Ready() <-chan struct{} {
	return a.ready
}

func (a *Agent) run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// TODO(stevvooe): Bring this back when we can extract Agent ID from
	// security configuration.
	// ctx = log.WithLogger(ctx, log.G(ctx).WithFields(logrus.Fields{
	// 	"agent.id": a.config.ID,
	// }))

	log.G(ctx).Debugf("(*Agent).run")
	defer log.G(ctx).Debugf("(*Agent).run exited")
	defer close(a.closed) // full shutdown.

	var (
		backoff    time.Duration
		session    = newSession(ctx, a, backoff) // start the initial session
		registered = session.registered
		ready      = a.ready // first session ready
	)

	// TODO(stevvooe): Read tasks known by executor associated with this node
	// and begin to manage them. This may be as simple as reporting their run
	// status and waiting for instruction from the manager.

	// TODO(stevvoe): Read tasks from disk store.

	for {
		select {
		case report := <-a.statusq:
			if err := a.handleTaskStatusReport(ctx, session, report); err != nil {
				log.G(ctx).WithError(err).Error("task status report handler failed")
			}
		case id := <-a.removedq:
			a.handleTaskRemoved(id)
		case msg := <-session.tasks:
			if err := a.handleTaskAssignment(ctx, msg.Tasks); err != nil {
				log.G(ctx).WithError(err).Error("task assignment failed")
			}
		case msg := <-session.messages:
			if err := a.handleSessionMessage(ctx, msg); err != nil {
				log.G(ctx).WithError(err).Error("session message handler failed")
			}
		case <-registered:
			if ready != nil {
				close(ready)
			}
			ready = nil
			log.G(ctx).Debugln("agent: registered")
			registered = nil // we only care about this once per session
			backoff = 0      // reset backoff
		case err := <-session.errs:
			// TODO(stevvooe): This may actually block if a session is closed
			// but no error was sent. Session.close must only be called here
			// for this to work.
			if err != nil {
				log.G(ctx).WithError(err).Error("agent: session failed")
				backoff = initialSessionFailureBackoff + 2*backoff
				if backoff > maxSessionFailureBackoff {
					backoff = maxSessionFailureBackoff
				}
			}

			if err := session.close(); err != nil {
				log.G(ctx).WithError(err).Error("agent: closing session failed")
			}
		case <-session.closed:
			log.G(ctx).Debugf("agent: rebuild session")

			// select a session registration delay from backoff range.
			delay := time.Duration(rand.Int63n(int64(backoff)))
			session = newSession(ctx, a, delay)
			registered = session.registered
		case <-a.stopped:
			// TODO(stevvooe): Wait on shutdown and cleanup. May need to pump
			// this loop a few times.
			return
		case <-ctx.Done():
			if a.err == nil {
				a.err = ctx.Err()
			}

			return
		}
	}
}

func (a *Agent) handleSessionMessage(ctx context.Context, message *api.SessionMessage) error {
	seen := map[api.Peer]struct{}{}
	for _, manager := range message.Managers {
		if manager.Peer.Addr == "" {
			log.G(ctx).WithField("manager.addr", manager.Peer.Addr).
				Warnf("skipping bad manager address")
			continue
		}

		a.config.Managers.Observe(*manager.Peer, int(manager.Weight))
		seen[*manager.Peer] = struct{}{}
	}

	if message.Node != nil {
		if a.node == nil || !nodesEqual(a.node, message.Node) {
			if a.config.NotifyRoleChange != nil {
				a.config.NotifyRoleChange <- message.Node.Spec.Role
			}
			a.node = message.Node.Copy()
			if err := a.config.Executor.Configure(ctx, a.node); err != nil {
				log.G(ctx).WithError(err).Error("node configure failed")
			}
		}
	}

	// prune managers not in list.
	for peer := range a.config.Managers.Weights() {
		if _, ok := seen[peer]; !ok {
			a.config.Managers.Remove(peer)
		}
	}

	if message.NetworkBootstrapKeys == nil {
		return nil
	}

	for _, key := range message.NetworkBootstrapKeys {
		same := false
		for _, agentKey := range a.keys {
			if agentKey.LamportTime == key.LamportTime {
				same = true
			}
		}
		if !same {
			a.keys = message.NetworkBootstrapKeys
			if err := a.config.Executor.SetNetworkBootstrapKeys(a.keys); err != nil {
				panic(fmt.Errorf("configuring network key failed"))
			}
		}
	}

	return nil
}

// assign the set of tasks to the agent. Any tasks on the agent currently that
// are not in the provided set will be terminated.
//
// This method run synchronously in the main session loop. It has direct access
// to fields and datastructures but must not block.
func (a *Agent) handleTaskAssignment(ctx context.Context, tasks []*api.Task) error {
	log.G(ctx).Debugf("(*Agent).handleTaskAssignment")

	assigned := map[string]*api.Task{}
	for _, task := range tasks {
		assigned[task.ID] = task
		ctx := log.WithLogger(ctx, log.G(ctx).WithField("task.id", task.ID))

		if _, ok := a.controllers[task.ID]; ok {
			if err := a.updateTask(ctx, task); err != nil {
				log.G(ctx).WithError(err).Error("task update failed")
			}
			continue
		}
		log.G(ctx).Debugf("assigned")
		if err := a.acceptTask(ctx, task); err != nil {
			log.G(ctx).WithError(err).Error("starting task controller failed")
			go func() {
				if err := a.report(ctx, task.ID, api.TaskStateRejected, "rejected task during assignment", err, nil); err != nil {
					log.G(ctx).WithError(err).Error("reporting task rejection failed")
				}
			}()
		}
	}

	for id, task := range a.tasks {
		if _, ok := assigned[id]; ok {
			continue
		}
		if report, ok := a.reports[id]; ok {
			if report.response != nil {
				report.response <- errTaskNotAssigned
			}
		}

		ctx := log.WithLogger(ctx, log.G(ctx).WithField("task.id", id))

		if err := a.removeTask(ctx, task); err != nil {
			log.G(ctx).WithError(err).Error("removing task failed")
		}
	}

	return nil
}

func (a *Agent) handleTaskStatusReport(ctx context.Context, session *session, report taskStatusReport) error {
	t, ok := a.tasks[report.taskID]
	if !ok {
		return errTaskUnknown
	}

	if report.state > api.TaskStateRunning || t.DesiredState >= report.state {
		return a.unblockTaskStatusReport(ctx, session, report)
	}

	if report.response != nil {
		// Save report to unblock it once the manager sets a high enough
		// DesiredState.
		if existingReport, ok := a.reports[report.taskID]; ok && existingReport.response != report.response {
			// It should not be possible for a task to be blocking on two
			// report() calls simultaneously.
			panic("duplicate blocked report")
		}
		a.reports[report.taskID] = report
	}
	return nil
}

func (a *Agent) handleTaskRemoved(id string) {
	delete(a.reports, id)
	delete(a.assigned, id)
	delete(a.controllers, id)
	delete(a.tasks, id)
	delete(a.shutdown, id)
}

func (a *Agent) unblockTaskStatusReport(ctx context.Context, session *session, report taskStatusReport) error {
	var respErr error
	status, err := a.updateStatus(ctx, report)
	if err != errTaskUnknown && err != errTaskDead && err != errTaskStatusUpdateNoChange && err != errTaskInvalidStateTransition {
		respErr = err
	}

	if report.response != nil {
		// this channel is always buffered.
		report.response <- respErr
		report.response = nil // clear response channel
		delete(a.reports, report.taskID)
	}

	if err != nil {
		return respErr
	}

	// TODO(stevvooe): Coalesce status updates.
	go func() {
		if err := session.sendTaskStatus(ctx, report.taskID, status); err != nil {
			if err == errTaskUnknown {
				return // dispatcher no longer cares about this task.
			}

			log.G(ctx).WithError(err).Error("sending task status update failed")

			time.Sleep(time.Second) // backoff for retry
			select {
			case a.statusq <- report: // queue for retry
			case <-a.closed:
			case <-ctx.Done():
			}
		}
	}()

	return nil
}

func (a *Agent) updateStatus(ctx context.Context, report taskStatusReport) (api.TaskStatus, error) {
	task, ok := a.tasks[report.taskID]
	if !ok {
		return api.TaskStatus{}, errTaskUnknown
	}

	status := task.Status
	original := task.Status
	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(
		logrus.Fields{
			"state.transition": fmt.Sprintf("%v->%v", original.State, report.state),
			"task.id":          report.taskID}))

	// validate transition only moves forward or updates fields
	if report.state < status.State && report.err == nil {
		log.G(ctx).Error("invalid transition")
		return api.TaskStatus{}, errTaskInvalidStateTransition
	}

	if report.err != nil {
		// If the error contains a ContainerStatus, use it.
		if exitErr, ok := report.err.(*exec.ExitError); ok {
			if report.cstatus == nil {
				report.cstatus = exitErr.ContainerStatus
			}
		}

		// If the task has been started, we return fail on error. If it has
		// not, we return rejected. While we don't do much differently for each
		// error type, it tells us the stage in which an error was encountered.
		switch status.State {
		case api.TaskStateNew, api.TaskStateAllocated,
			api.TaskStateAssigned, api.TaskStateAccepted,
			api.TaskStatePreparing:
			status.State = api.TaskStateRejected
		case api.TaskStateReady, api.TaskStateStarting,
			api.TaskStateRunning:
			status.State = api.TaskStateFailed
		case api.TaskStateCompleted, api.TaskStateShutdown,
			api.TaskStateFailed, api.TaskStateRejected:
			// noop when we get an error in these states
		default:
			status.Err = report.err.Error()
		}
	} else {
		status.State = report.state
	}

	tsp, err := ptypes.TimestampProto(report.timestamp)
	if err != nil {
		return api.TaskStatus{}, err
	}

	status.Timestamp = tsp
	status.Message = report.message
	if report.cstatus != nil {
		status.RuntimeStatus = &api.TaskStatus_Container{
			Container: report.cstatus,
		}
	} else {
		status.RuntimeStatus = original.RuntimeStatus
	}

	if reflect.DeepEqual(status, original) {
		return api.TaskStatus{}, errTaskStatusUpdateNoChange
	}

	log.G(ctx).WithFields(logrus.Fields{
		"state.message": status.Message,
	}).Infof("task status updated")

	switch status.State {
	case api.TaskStateNew, api.TaskStateAllocated,
		api.TaskStateAssigned, api.TaskStateAccepted,
		api.TaskStatePreparing, api.TaskStateReady,
		api.TaskStateStarting, api.TaskStateRunning:
	case api.TaskStateShutdown, api.TaskStateCompleted,
		api.TaskStateFailed, api.TaskStateRejected:
		delete(a.shutdown, report.taskID) // cleanup shutdown entry
	}

	task.Status = status // actually write out the task status.
	return status, nil
}

func (a *Agent) acceptTask(ctx context.Context, task *api.Task) error {
	a.tasks[task.ID] = task
	a.assigned[task.ID] = task

	ctlr, err := a.config.Executor.Controller(task.Copy())
	if err != nil {
		log.G(ctx).WithError(err).Error("controller resolution failed")
		return err
	}

	a.controllers[task.ID] = ctlr
	reporter := a.reporter(ctx, task)
	taskID := task.ID

	go func() {
		if err := reporter.Report(ctx, api.TaskStateAccepted, "accepted", nil); err != nil {
			// TODO(stevvooe): What to do here? should be a rare error or never happen
			log.G(ctx).WithError(err).Error("reporting accepted status")
			return
		}

		if err := exec.Run(ctx, ctlr, reporter); err != nil {
			log.G(ctx).WithError(err).Error("task run failed")
			if err := a.report(ctx, taskID, api.TaskStateFailed, "execution failed", err, nil); err != nil {
				log.G(ctx).WithError(err).Error("reporting task run error failed")
			}
			return
		}
	}()

	return nil
}

func (a *Agent) updateTask(ctx context.Context, t *api.Task) error {
	if _, ok := a.assigned[t.ID]; !ok {
		return errTaskNotAssigned
	}

	original := a.tasks[t.ID]
	t.Status = original.Status // don't overwrite agent's task status
	a.tasks[t.ID] = t
	a.assigned[t.ID] = t

	if report, ok := a.reports[t.ID]; ok {
		// Try to unblock report if the new DesiredState allows it
		go func() {
			select {
			case a.statusq <- report:
			case <-a.closed:
			case <-ctx.Done():
			}
		}()
	}

	if !tasksEqual(t, original) {
		ctlr := a.controllers[t.ID]
		t := t.Copy()
		// propagate the update if there are actual changes
		go func() {
			if err := ctlr.Update(ctx, t); err != nil {
				log.G(ctx).WithError(err).Error("propagating task update failed")
			}
		}()
	}

	if t.DesiredState > api.TaskStateRunning && t.Status.State < api.TaskStateCompleted {
		if err := a.shutdownTask(ctx, t); err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) shutdownTask(ctx context.Context, t *api.Task) error {
	log.G(ctx).Debugf("(*Agent).shutdownTask")

	if _, ok := a.shutdown[t.ID]; ok {
		return nil // already shutdown
	}
	a.shutdown[t.ID] = struct{}{}

	var (
		ctlr = a.controllers[t.ID]
	)

	go func() {
		reporter := a.reporter(ctx, t)
		for {
			if err := exec.Shutdown(ctx, ctlr, reporter); err != nil {
				if err == exec.ErrControllerClosed {
					return
				}

				log.G(ctx).WithError(err).Error("failed to shutdown task")
				continue // retry until dead
			}

			return // success
		}
	}()

	return nil
}

func (a *Agent) removeTask(ctx context.Context, t *api.Task) error {
	log.G(ctx).Debugf("(*Agent).removeTask")

	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.remove[t.ID]; ok {
		return nil // already removing
	}
	a.remove[t.ID] = struct{}{}

	t = t.Copy()
	ctlr := a.controllers[t.ID]
	go func() {
		for i := 0; i < 10; i++ {
			if err := ctlr.Remove(ctx); err != nil {
				log.G(ctx).WithError(err).Error("remove failed")
				time.Sleep(1 * time.Second)
				continue
			}

			select {
			case a.removedq <- t.ID:
			case <-a.closed:
				return
			case <-ctx.Done():
				return
			}

			break
		}

		a.mu.Lock()
		delete(a.remove, t.ID)
		a.mu.Unlock()
	}()

	return nil
}

type taskStatusReport struct {
	timestamp time.Time
	taskID    string
	state     api.TaskState
	cstatus   *api.ContainerStatus
	message   string
	err       error
	response  chan error
}

func (a *Agent) report(ctx context.Context, taskID string, state api.TaskState, msg string, err error, cstatus *api.ContainerStatus) error {
	log.G(ctx).Debugf("(*Agent).report")
	response := make(chan error, 1)

	select {
	case a.statusq <- taskStatusReport{
		timestamp: time.Now(),
		taskID:    taskID,
		state:     state,
		cstatus:   cstatus,
		message:   msg,
		err:       err,
		response:  response,
	}:
		select {
		case err := <-response:
			return err
		case <-a.closed:
			return ErrAgentClosed
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-a.closed:
		return ErrAgentClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Agent) reporter(ctx context.Context, t *api.Task) exec.Reporter {
	id := t.ID
	return reporterFunc(func(ctx context.Context, state api.TaskState, msg string, cstatus *api.ContainerStatus) error {
		return a.report(ctx, id, state, msg, nil, cstatus)
	})
}

type reporterFunc func(ctx context.Context, state api.TaskState, msg string, status *api.ContainerStatus) error

func (fn reporterFunc) Report(ctx context.Context, state api.TaskState, msg string, status *api.ContainerStatus) error {
	return fn(ctx, state, msg, status)
}

// tasksEqual returns true if the tasks are functionaly equal, ignoring status,
// version and other superfluous fields.
//
// This used to decide whether or not to propagate a task update to a controller.
func tasksEqual(a, b *api.Task) bool {
	a, b = a.Copy(), b.Copy()

	a.Status, b.Status = api.TaskStatus{}, api.TaskStatus{}
	a.Meta, b.Meta = api.Meta{}, api.Meta{}

	return reflect.DeepEqual(a, b)
}

// nodesEqual returns true if the node states are functionaly equal, ignoring status,
// version and other superfluous fields.
//
// This used to decide whether or not to propagate a node update to executor.
func nodesEqual(a, b *api.Node) bool {
	a, b = a.Copy(), b.Copy()

	a.Status, b.Status = api.NodeStatus{}, api.NodeStatus{}
	a.Meta, b.Meta = api.Meta{}, api.Meta{}

	return reflect.DeepEqual(a, b)
}
