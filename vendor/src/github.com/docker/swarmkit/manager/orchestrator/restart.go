package orchestrator

import (
	"container/list"
	"sync"
	"time"

	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"golang.org/x/net/context"
)

const defaultOldTaskTimeout = time.Minute
const defaultRestartDelay = 5 * time.Second

type restartedInstance struct {
	timestamp time.Time
}

type instanceRestartInfo struct {
	// counter of restarts for this instance.
	totalRestarts uint64
	// Linked list of restartedInstance structs. Only used when
	// Restart.MaxAttempts and Restart.Window are both
	// nonzero.
	restartedInstances *list.List
}

type delayedStart struct {
	// cancel is called to cancel the delayed start.
	cancel func()
	doneCh chan struct{}

	// waiter is set to true if the next restart is waiting for this delay
	// to complete.
	waiter bool
}

// RestartSupervisor initiates and manages restarts. It's responsible for
// delaying restarts when applicable.
type RestartSupervisor struct {
	mu               sync.Mutex
	store            *store.MemoryStore
	delays           map[string]*delayedStart
	history          map[instanceTuple]*instanceRestartInfo
	historyByService map[string]map[instanceTuple]struct{}
	taskTimeout      time.Duration
}

// NewRestartSupervisor creates a new RestartSupervisor.
func NewRestartSupervisor(store *store.MemoryStore) *RestartSupervisor {
	return &RestartSupervisor{
		store:            store,
		delays:           make(map[string]*delayedStart),
		history:          make(map[instanceTuple]*instanceRestartInfo),
		historyByService: make(map[string]map[instanceTuple]struct{}),
		taskTimeout:      defaultOldTaskTimeout,
	}
}

func (r *RestartSupervisor) waitRestart(ctx context.Context, oldDelay *delayedStart, cluster *api.Cluster, taskID string) {
	// Wait for the last restart delay to elapse.
	select {
	case <-oldDelay.doneCh:
	case <-ctx.Done():
		return
	}

	// Start the next restart
	err := r.store.Update(func(tx store.Tx) error {
		t := store.GetTask(tx, taskID)
		if t == nil {
			return nil
		}
		service := store.GetService(tx, t.ServiceID)
		if service == nil {
			return nil
		}
		return r.Restart(ctx, tx, cluster, service, *t)
	})

	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to restart task after waiting for previous restart")
	}
}

// Restart initiates a new task to replace t if appropriate under the service's
// restart policy.
func (r *RestartSupervisor) Restart(ctx context.Context, tx store.Tx, cluster *api.Cluster, service *api.Service, t api.Task) error {
	// TODO(aluzzardi): This function should not depend on `service`.

	// Is the old task still in the process of restarting? If so, wait for
	// its restart delay to elapse, to avoid tight restart loops (for
	// example, when the image doesn't exist).
	r.mu.Lock()
	oldDelay, ok := r.delays[t.ID]
	if ok {
		if !oldDelay.waiter {
			oldDelay.waiter = true
			go r.waitRestart(ctx, oldDelay, cluster, t.ID)
		}
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	t.DesiredState = api.TaskStateShutdown
	err := store.UpdateTask(tx, &t)
	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to set task desired state to dead")
		return err
	}

	if !r.shouldRestart(ctx, &t, service) {
		return nil
	}

	var restartTask *api.Task

	if isReplicatedService(service) {
		restartTask = newTask(cluster, service, t.Slot)
	} else if isGlobalService(service) {
		restartTask = newTask(cluster, service, 0)
		restartTask.NodeID = t.NodeID
	} else {
		log.G(ctx).Error("service not supported by restart supervisor")
		return nil
	}

	n := store.GetNode(tx, t.NodeID)

	restartTask.DesiredState = api.TaskStateReady

	var restartDelay time.Duration
	// Restart delay is not applied to drained nodes
	if n == nil || n.Spec.Availability != api.NodeAvailabilityDrain {
		if t.Spec.Restart != nil && t.Spec.Restart.Delay != nil {
			var err error
			restartDelay, err = ptypes.Duration(t.Spec.Restart.Delay)
			if err != nil {
				log.G(ctx).WithError(err).Error("invalid restart delay; using default")
				restartDelay = defaultRestartDelay
			}
		} else {
			restartDelay = defaultRestartDelay
		}
	}

	waitStop := true

	// Normally we wait for the old task to stop running, but we skip this
	// if the old task is already dead or the node it's assigned to is down.
	if (n != nil && n.Status.State == api.NodeStatus_DOWN) || t.Status.State > api.TaskStateRunning {
		waitStop = false
	}

	if err := store.CreateTask(tx, restartTask); err != nil {
		log.G(ctx).WithError(err).WithField("task.id", restartTask.ID).Error("task create failed")
		return err
	}

	r.recordRestartHistory(restartTask)

	r.DelayStart(ctx, tx, &t, restartTask.ID, restartDelay, waitStop)
	return nil
}

func (r *RestartSupervisor) shouldRestart(ctx context.Context, t *api.Task, service *api.Service) bool {
	// TODO(aluzzardi): This function should not depend on `service`.

	condition := restartCondition(t)

	if condition != api.RestartOnAny &&
		(condition != api.RestartOnFailure || t.Status.State == api.TaskStateCompleted) {
		return false
	}

	if t.Spec.Restart == nil || t.Spec.Restart.MaxAttempts == 0 {
		return true
	}

	instanceTuple := instanceTuple{
		instance:  t.Slot,
		serviceID: t.ServiceID,
	}

	// Instance is not meaningful for "global" tasks, so they need to be
	// indexed by NodeID.
	if isGlobalService(service) {
		instanceTuple.nodeID = t.NodeID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	restartInfo := r.history[instanceTuple]
	if restartInfo == nil {
		return true
	}

	if t.Spec.Restart.Window == nil || (t.Spec.Restart.Window.Seconds == 0 && t.Spec.Restart.Window.Nanos == 0) {
		return restartInfo.totalRestarts < t.Spec.Restart.MaxAttempts
	}

	if restartInfo.restartedInstances == nil {
		return true
	}

	window, err := ptypes.Duration(t.Spec.Restart.Window)
	if err != nil {
		log.G(ctx).WithError(err).Error("invalid restart lookback window")
		return restartInfo.totalRestarts < t.Spec.Restart.MaxAttempts
	}
	lookback := time.Now().Add(-window)

	var next *list.Element
	for e := restartInfo.restartedInstances.Front(); e != nil; e = next {
		next = e.Next()

		if e.Value.(restartedInstance).timestamp.After(lookback) {
			break
		}
		restartInfo.restartedInstances.Remove(e)
	}

	numRestarts := uint64(restartInfo.restartedInstances.Len())

	if numRestarts == 0 {
		restartInfo.restartedInstances = nil
	}

	return numRestarts < t.Spec.Restart.MaxAttempts
}

func (r *RestartSupervisor) recordRestartHistory(restartTask *api.Task) {
	if restartTask.Spec.Restart == nil || restartTask.Spec.Restart.MaxAttempts == 0 {
		// No limit on the number of restarts, so no need to record
		// history.
		return
	}
	tuple := instanceTuple{
		instance:  restartTask.Slot,
		serviceID: restartTask.ServiceID,
		nodeID:    restartTask.NodeID,
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.history[tuple] == nil {
		r.history[tuple] = &instanceRestartInfo{}
	}

	restartInfo := r.history[tuple]
	restartInfo.totalRestarts++

	if r.historyByService[restartTask.ServiceID] == nil {
		r.historyByService[restartTask.ServiceID] = make(map[instanceTuple]struct{})
	}
	r.historyByService[restartTask.ServiceID][tuple] = struct{}{}

	if restartTask.Spec.Restart.Window != nil && (restartTask.Spec.Restart.Window.Seconds != 0 || restartTask.Spec.Restart.Window.Nanos != 0) {
		if restartInfo.restartedInstances == nil {
			restartInfo.restartedInstances = list.New()
		}

		restartedInstance := restartedInstance{
			timestamp: time.Now(),
		}

		restartInfo.restartedInstances.PushBack(restartedInstance)
	}
}

// DelayStart starts a timer that moves the task from READY to RUNNING once:
// - The restart delay has elapsed (if applicable)
// - The old task that it's replacing has stopped running (or this times out)
// It must be called during an Update transaction to ensure that it does not
// miss events. The purpose of the store.Tx argument is to avoid accidental
// calls outside an Update transaction.
func (r *RestartSupervisor) DelayStart(ctx context.Context, _ store.Tx, oldTask *api.Task, newTaskID string, delay time.Duration, waitStop bool) <-chan struct{} {
	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan struct{})

	r.mu.Lock()
	for {
		oldDelay, ok := r.delays[newTaskID]
		if !ok {
			break
		}
		oldDelay.cancel()
		r.mu.Unlock()
		// Note that this channel read should only block for a very
		// short time, because we cancelled the existing delay and
		// that should cause it to stop immediately.
		<-oldDelay.doneCh
		r.mu.Lock()
	}
	r.delays[newTaskID] = &delayedStart{cancel: cancel, doneCh: doneCh}
	r.mu.Unlock()

	var watch chan events.Event
	cancelWatch := func() {}

	if waitStop && oldTask != nil {
		// Wait for either the old task to complete, or the old task's
		// node to become unavailable.
		watch, cancelWatch = state.Watch(
			r.store.WatchQueue(),
			state.EventUpdateTask{
				Task:   &api.Task{ID: oldTask.ID, Status: api.TaskStatus{State: api.TaskStateRunning}},
				Checks: []state.TaskCheckFunc{state.TaskCheckID, state.TaskCheckStateGreaterThan},
			},
			state.EventUpdateNode{
				Node:   &api.Node{ID: oldTask.NodeID, Status: api.NodeStatus{State: api.NodeStatus_DOWN}},
				Checks: []state.NodeCheckFunc{state.NodeCheckID, state.NodeCheckState},
			},
			state.EventDeleteNode{
				Node:   &api.Node{ID: oldTask.NodeID},
				Checks: []state.NodeCheckFunc{state.NodeCheckID},
			},
		)
	}

	go func() {
		defer func() {
			cancelWatch()
			r.mu.Lock()
			delete(r.delays, newTaskID)
			r.mu.Unlock()
			close(doneCh)
		}()

		oldTaskTimeout := time.After(r.taskTimeout)

		// Wait for the delay to elapse, if one is specified.
		if delay != 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}

		if waitStop {
			select {
			case <-watch:
			case <-oldTaskTimeout:
			case <-ctx.Done():
				return
			}
		}

		err := r.store.Update(func(tx store.Tx) error {
			err := r.StartNow(tx, newTaskID)
			if err != nil {
				log.G(ctx).WithError(err).WithField("task.id", newTaskID).Error("moving task out of delayed state failed")
			}
			return nil
		})
		if err != nil {
			log.G(ctx).WithError(err).WithField("task.id", newTaskID).Error("task restart transaction failed")
		}
	}()

	return doneCh
}

// StartNow moves the task into the RUNNING state so it will proceed to start
// up.
func (r *RestartSupervisor) StartNow(tx store.Tx, taskID string) error {
	t := store.GetTask(tx, taskID)
	if t == nil || t.DesiredState >= api.TaskStateRunning {
		return nil
	}
	t.DesiredState = api.TaskStateRunning
	return store.UpdateTask(tx, t)
}

// Cancel cancels a pending restart.
func (r *RestartSupervisor) Cancel(taskID string) {
	r.mu.Lock()
	delay, ok := r.delays[taskID]
	r.mu.Unlock()

	if !ok {
		return
	}

	delay.cancel()
	<-delay.doneCh
}

// CancelAll aborts all pending restarts and waits for any instances of
// StartNow that have already triggered to complete.
func (r *RestartSupervisor) CancelAll() {
	var cancelled []delayedStart

	r.mu.Lock()
	for _, delay := range r.delays {
		delay.cancel()
	}
	r.mu.Unlock()

	for _, delay := range cancelled {
		<-delay.doneCh
	}
}

// ClearServiceHistory forgets restart history related to a given service ID.
func (r *RestartSupervisor) ClearServiceHistory(serviceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tuples := r.historyByService[serviceID]
	if tuples == nil {
		return
	}

	delete(r.historyByService, serviceID)

	for t := range tuples {
		delete(r.history, t)
	}
}
