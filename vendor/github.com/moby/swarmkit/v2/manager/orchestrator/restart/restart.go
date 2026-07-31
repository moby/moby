package restart

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/docker/go-events"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/api/defaults"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/orchestrator"
	"github.com/moby/swarmkit/v2/manager/state"
	"github.com/moby/swarmkit/v2/manager/state/store"
)

const defaultOldTaskTimeout = time.Minute

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
	// Why is specVersion in this structure and not in the map key? While
	// putting it in the key would be a very simple solution, it wouldn't
	// be easy to clean up map entries corresponding to old specVersions.
	// Making the key version-agnostic and clearing the value whenever the
	// version changes avoids the issue of stale map entries for old
	// versions.
	specVersion uint64
}

type delayedStart struct {
	// cancel is called to cancel the delayed start.
	cancel func()
	doneCh chan struct{}

	// waiter is set to true if the next restart is waiting for this delay
	// to complete.
	waiter bool
}

// SupervisorInterface is an interface implemented by the Supervisor. It exists
// to make testing easier, by allowing the restart supervisor to be mocked or
// faked where desired.
type SupervisorInterface interface {
	Restart(context.Context, store.Tx, *api.Cluster, *api.Service, *api.Task) error
	UpdatableTasksInSlot(context.Context, orchestrator.Slot, *api.Service) orchestrator.Slot
	RecordRestartHistory(orchestrator.SlotTuple, *api.Task)
	DelayStart(context.Context, store.Tx, *api.Task, string, time.Duration, bool) <-chan struct{}
	StartNow(store.Tx, string) error
	Cancel(string)
	CancelAll()
	ClearServiceHistory(string)
}

// Supervisor initiates and manages restarts. It's responsible for
// delaying restarts when applicable.
type Supervisor struct {
	mu               sync.Mutex
	store            *store.MemoryStore
	delays           map[string]*delayedStart
	historyByService map[string]map[orchestrator.SlotTuple]*instanceRestartInfo
	TaskTimeout      time.Duration
}

// NewSupervisor creates a new RestartSupervisor.
func NewSupervisor(store *store.MemoryStore) *Supervisor {
	return &Supervisor{
		store:            store,
		delays:           make(map[string]*delayedStart),
		historyByService: make(map[string]map[orchestrator.SlotTuple]*instanceRestartInfo),
		TaskTimeout:      defaultOldTaskTimeout,
	}
}

func (r *Supervisor) waitRestart(ctx context.Context, oldDelay *delayedStart, cluster *api.Cluster, taskID string) {
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
		if t.DesiredState > api.TaskState_RUNNING {
			return nil
		}
		service := store.GetService(tx, t.ServiceId)
		if service == nil {
			return nil
		}
		return r.Restart(ctx, tx, cluster, service, t)
	})

	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to restart task after waiting for previous restart")
	}
}

// Restart initiates a new task to replace t if appropriate under the service's
// restart policy.
func (r *Supervisor) Restart(ctx context.Context, tx store.Tx, cluster *api.Cluster, service *api.Service, t *api.Task) error {
	// TODO(aluzzardi): This function should not depend on `service`.

	// Is the old task still in the process of restarting? If so, wait for
	// its restart delay to elapse, to avoid tight restart loops (for
	// example, when the image doesn't exist).
	r.mu.Lock()
	oldDelay, ok := r.delays[t.Id]
	if ok {
		if !oldDelay.waiter {
			oldDelay.waiter = true
			go r.waitRestart(ctx, oldDelay, cluster, t.Id)
		}
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	// Sanity check: was the task shut down already by a separate call to
	// Restart? If so, we must avoid restarting it, because this will create
	// an extra task. This should never happen unless there is a bug.
	if t.DesiredState > api.TaskState_COMPLETE {
		return errors.New("Restart called on task that was already shut down")
	}

	t.DesiredState = api.TaskState_SHUTDOWN
	err := store.UpdateTask(tx, t)
	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to set task desired state to dead")
		return err
	}

	if !r.shouldRestart(ctx, t, service) {
		return nil
	}

	var restartTask *api.Task

	if orchestrator.IsReplicatedService(service) || orchestrator.IsReplicatedJob(service) {
		restartTask = orchestrator.NewTask(cluster, service, t.Slot, "")
	} else if orchestrator.IsGlobalService(service) || orchestrator.IsGlobalJob(service) {
		restartTask = orchestrator.NewTask(cluster, service, 0, t.NodeId)
	} else {
		log.G(ctx).Error("service not supported by restart supervisor")
		return nil
	}

	if orchestrator.IsReplicatedJob(service) || orchestrator.IsGlobalJob(service) {
		restartTask.JobIteration = &api.Version{
			Index: service.JobStatus.JobIteration.Index,
		}
	}

	n := store.GetNode(tx, t.NodeId)

	restartTask.DesiredState = api.TaskState_READY

	var restartDelay time.Duration
	// Restart delay is not applied to drained nodes
	if n == nil || n.Spec.GetAvailability() != api.NodeSpec_DRAIN {
		// AsDuration cannot fail, so validate explicitly to keep falling back
		// to the default for a malformed delay.
		if d := t.Spec.GetRestart().GetDelay(); d == nil {
			restartDelay = defaults.Service.Task.Restart.Delay.AsDuration()
		} else if d.IsValid() {
			restartDelay = d.AsDuration()
		} else {
			log.G(ctx).Error("invalid restart delay; using default")
			restartDelay = defaults.Service.Task.Restart.Delay.AsDuration()
		}
	}

	waitStop := true

	// Normally we wait for the old task to stop running, but we skip this
	// if the old task is already dead or the node it's assigned to is down.
	if (n != nil && n.Status.GetState() == api.NodeStatus_DOWN) || t.Status.GetState() > api.TaskState_RUNNING {
		waitStop = false
	}

	if err := store.CreateTask(tx, restartTask); err != nil {
		log.G(ctx).WithError(err).WithField("task.id", restartTask.Id).Error("task create failed")
		return err
	}

	tuple := orchestrator.SlotTuple{
		Slot:      restartTask.Slot,
		ServiceID: restartTask.ServiceId,
		NodeID:    restartTask.NodeId,
	}
	r.RecordRestartHistory(tuple, restartTask)

	r.DelayStart(ctx, tx, t, restartTask.Id, restartDelay, waitStop)
	return nil
}

// shouldRestart returns true if a task should be restarted according to the
// restart policy.
func (r *Supervisor) shouldRestart(ctx context.Context, t *api.Task, service *api.Service) bool {
	// TODO(aluzzardi): This function should not depend on `service`.
	// There are 3 possible restart policies.
	switch orchestrator.RestartCondition(t) {
	case api.RestartPolicy_ANY:
		// we will be restarting, we just need to do a few more checks.
		// however, if the task belongs to a job, then we will treat
		// RestartOnAny the same as RestartOnFailure, as it would be
		// nonsensical to restart completed jobs.
		if orchestrator.IsReplicatedJob(service) || orchestrator.IsGlobalJob(service) {
			// it'd be nice to put a fallthrough here, but we can't fallthrough
			// from inside of an if statement.
			if t.Status.GetState() == api.TaskState_COMPLETE {
				return false
			}
		}
	case api.RestartPolicy_ON_FAILURE:
		// we won't restart if the task is in TaskStateCompleted, as this is a
		// not a failed state -- it indicates that the task exited with 0
		if t.Status.GetState() == api.TaskState_COMPLETE {
			return false
		}
	case api.RestartPolicy_NONE:
		// RestartOnNone means we just don't restart, ever
		return false
	}

	if t.Spec.GetRestart() == nil || t.Spec.GetRestart().GetMaxAttempts() == 0 {
		return true
	}

	instanceTuple := orchestrator.SlotTuple{
		Slot:      t.Slot,
		ServiceID: t.ServiceId,
	}

	// Slot is not meaningful for "global" tasks, so they need to be
	// indexed by NodeID.
	if orchestrator.IsGlobalService(service) {
		instanceTuple.NodeID = t.NodeId
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	restartInfo := r.historyByService[t.ServiceId][instanceTuple]
	if restartInfo == nil || (t.SpecVersion != nil && t.SpecVersion.GetIndex() != restartInfo.specVersion) {
		return true
	}

	if t.Spec.GetRestart().GetWindow() == nil || (t.Spec.GetRestart().GetWindow().Seconds == 0 && t.Spec.GetRestart().GetWindow().Nanos == 0) {
		return restartInfo.totalRestarts < t.Spec.GetRestart().GetMaxAttempts()
	}

	if restartInfo.restartedInstances == nil {
		return true
	}

	// AsDuration cannot fail, so validate explicitly to keep the fallback for
	// a window we cannot make sense of.
	if !t.Spec.GetRestart().GetWindow().IsValid() {
		log.G(ctx).Error("invalid restart lookback window")
		return restartInfo.totalRestarts < t.Spec.GetRestart().GetMaxAttempts()
	}
	window := t.Spec.GetRestart().GetWindow().AsDuration()

	var timestamp time.Time
	// Prefer the manager's timestamp over the agent's, since manager
	// clocks are more trustworthy.
	if applied := t.Status.GetAppliedAt(); applied != nil {
		if !applied.IsValid() {
			log.G(ctx).Error("invalid task status AppliedAt timestamp")
			return restartInfo.totalRestarts < t.Spec.GetRestart().GetMaxAttempts()
		}
		timestamp = applied.AsTime()
	} else {
		completed := t.Status.GetTimestamp()
		if completed == nil || !completed.IsValid() {
			log.G(ctx).Error("invalid task completion timestamp")
			return restartInfo.totalRestarts < t.Spec.GetRestart().GetMaxAttempts()
		}
		timestamp = completed.AsTime()
	}
	lookback := timestamp.Add(-window)

	numRestarts := uint64(restartInfo.restartedInstances.Len())

	// Disregard any restarts that happened before the lookback window,
	// and remove them from the linked list since they will no longer
	// be relevant to figuring out if tasks should be restarted going
	// forward.
	var next *list.Element
	for e := restartInfo.restartedInstances.Front(); e != nil; e = next {
		next = e.Next()

		if e.Value.(restartedInstance).timestamp.After(lookback) {
			break
		}
		restartInfo.restartedInstances.Remove(e)
		numRestarts--
	}

	// Ignore restarts that didn't happen before the task we're looking at.
	for e2 := restartInfo.restartedInstances.Back(); e2 != nil; e2 = e2.Prev() {
		if e2.Value.(restartedInstance).timestamp.Before(timestamp) {
			break
		}
		numRestarts--
	}

	if restartInfo.restartedInstances.Len() == 0 {
		restartInfo.restartedInstances = nil
	}

	return numRestarts < t.Spec.GetRestart().GetMaxAttempts()
}

// UpdatableTasksInSlot returns the set of tasks that should be passed to the
// updater from this slot, or an empty slice if none should be.  An updatable
// slot has either at least one task that with desired state <= RUNNING, or its
// most recent task has stopped running and should not be restarted. The latter
// case is for making sure that tasks that shouldn't normally be restarted will
// still be handled by rolling updates when they become outdated.  There is a
// special case for rollbacks to make sure that a rollback always takes the
// service to a converged state, instead of ignoring tasks with the original
// spec that stopped running and shouldn't be restarted according to the
// restart policy.
func (r *Supervisor) UpdatableTasksInSlot(ctx context.Context, slot orchestrator.Slot, service *api.Service) orchestrator.Slot {
	if len(slot) < 1 {
		return nil
	}

	var updatable orchestrator.Slot
	for _, t := range slot {
		if t.DesiredState <= api.TaskState_RUNNING {
			updatable = append(updatable, t)
		}
	}
	if len(updatable) > 0 {
		return updatable
	}

	if service.UpdateStatus != nil && service.UpdateStatus.State == api.UpdateStatus_ROLLBACK_STARTED {
		return nil
	}

	// Find most recent task
	byTimestamp := orchestrator.TasksByTimestamp(slot)
	newestIndex := 0
	for i := 1; i < len(slot); i++ {
		if byTimestamp.Less(newestIndex, i) {
			newestIndex = i
		}
	}

	if !r.shouldRestart(ctx, slot[newestIndex], service) {
		return orchestrator.Slot{slot[newestIndex]}
	}
	return nil
}

// RecordRestartHistory updates the historyByService map to reflect the restart
// of restartedTask.
func (r *Supervisor) RecordRestartHistory(tuple orchestrator.SlotTuple, replacementTask *api.Task) {
	if replacementTask.Spec.GetRestart() == nil || replacementTask.Spec.GetRestart().GetMaxAttempts() == 0 {
		// No limit on the number of restarts, so no need to record
		// history.
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	serviceID := replacementTask.ServiceId
	if r.historyByService[serviceID] == nil {
		r.historyByService[serviceID] = make(map[orchestrator.SlotTuple]*instanceRestartInfo)
	}
	if r.historyByService[serviceID][tuple] == nil {
		r.historyByService[serviceID][tuple] = &instanceRestartInfo{}
	}

	restartInfo := r.historyByService[serviceID][tuple]

	if replacementTask.SpecVersion != nil && replacementTask.SpecVersion.GetIndex() != restartInfo.specVersion {
		// This task has a different SpecVersion from the one we're
		// tracking. Most likely, the service was updated. Past failures
		// shouldn't count against the new service definition, so clear
		// the history for this instance.
		*restartInfo = instanceRestartInfo{
			specVersion: replacementTask.SpecVersion.GetIndex(),
		}
	}

	restartInfo.totalRestarts++

	if replacementTask.Spec.GetRestart().GetWindow() != nil && (replacementTask.Spec.GetRestart().GetWindow().Seconds != 0 || replacementTask.Spec.GetRestart().GetWindow().Nanos != 0) {
		if restartInfo.restartedInstances == nil {
			restartInfo.restartedInstances = list.New()
		}

		timestamp := time.Now()
		if created := replacementTask.Meta.GetCreatedAt(); created.IsValid() {
			timestamp = created.AsTime()
		}

		restartedInstance := restartedInstance{
			timestamp: timestamp,
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
func (r *Supervisor) DelayStart(ctx context.Context, _ store.Tx, oldTask *api.Task, newTaskID string, delay time.Duration, waitStop bool) <-chan struct{} {
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

	waitForTask := waitStop && oldTask != nil && oldTask.Status.GetState() <= api.TaskState_RUNNING

	if waitForTask {
		// Wait for either the old task to complete, or the old task's
		// node to become unavailable.
		watch, cancelWatch = state.Watch(
			r.store.WatchQueue(),
			api.EventUpdateTask{
				Task:   &api.Task{Id: oldTask.Id, Status: &api.TaskStatus{State: api.TaskState_RUNNING}},
				Checks: []api.TaskCheckFunc{api.TaskCheckID, state.TaskCheckStateGreaterThan},
			},
			api.EventUpdateNode{
				Node:   &api.Node{Id: oldTask.NodeId, Status: &api.NodeStatus{State: api.NodeStatus_DOWN}},
				Checks: []api.NodeCheckFunc{api.NodeCheckID, state.NodeCheckState},
			},
			api.EventDeleteNode{
				Node:   &api.Node{Id: oldTask.NodeId},
				Checks: []api.NodeCheckFunc{api.NodeCheckID},
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

		oldTaskTimer := time.NewTimer(r.TaskTimeout)
		defer oldTaskTimer.Stop()

		// Wait for the delay to elapse, if one is specified.
		if delay != 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}

		if waitForTask {
			select {
			case <-watch:
			case <-oldTaskTimer.C:
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
func (r *Supervisor) StartNow(tx store.Tx, taskID string) error {
	t := store.GetTask(tx, taskID)
	if t == nil || t.DesiredState >= api.TaskState_RUNNING {
		return nil
	}

	// only tasks belonging to jobs will have a JobIteration, so this can be
	// used to distinguish whether this is a job task without looking at the
	// service.
	if t.JobIteration != nil {
		t.DesiredState = api.TaskState_COMPLETE
	} else {
		t.DesiredState = api.TaskState_RUNNING
	}
	return store.UpdateTask(tx, t)
}

// Cancel cancels a pending restart.
func (r *Supervisor) Cancel(taskID string) {
	r.mu.Lock()
	delay, ok := r.delays[taskID]
	r.mu.Unlock()

	if !ok {
		return
	}

	delay.cancel()
	<-delay.doneCh
}

// CancelAll aborts all pending restarts
func (r *Supervisor) CancelAll() {
	r.mu.Lock()
	for _, delay := range r.delays {
		delay.cancel()
	}
	r.mu.Unlock()
}

// ClearServiceHistory forgets restart history related to a given service ID.
func (r *Supervisor) ClearServiceHistory(serviceID string) {
	r.mu.Lock()
	delete(r.historyByService, serviceID)
	r.mu.Unlock()
}
