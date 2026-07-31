package update

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/docker/go-events"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/api/defaults"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/orchestrator"
	"github.com/moby/swarmkit/v2/manager/orchestrator/restart"
	"github.com/moby/swarmkit/v2/manager/state"
	"github.com/moby/swarmkit/v2/manager/state/store"
	"github.com/moby/swarmkit/v2/protobuf/ptypes"
	"github.com/moby/swarmkit/v2/watch"
)

// Supervisor supervises a set of updates. It's responsible for keeping track of updates,
// shutting them down and replacing them.
type Supervisor struct {
	store    *store.MemoryStore
	restarts *restart.Supervisor
	updates  map[string]*Updater
	l        sync.Mutex
}

// NewSupervisor creates a new UpdateSupervisor.
func NewSupervisor(store *store.MemoryStore, restartSupervisor *restart.Supervisor) *Supervisor {
	return &Supervisor{
		store:    store,
		updates:  make(map[string]*Updater),
		restarts: restartSupervisor,
	}
}

// Update starts an Update of `slots` belonging to `service` in the background
// and returns immediately. Each slot contains a group of one or more tasks
// occupying the same slot (replicated service) or node (global service). There
// may be more than one task per slot in cases where an update is in progress
// and the new task was started before the old one was shut down. If an update
// for that service was already in progress, it will be cancelled before the
// new one starts.
func (u *Supervisor) Update(ctx context.Context, cluster *api.Cluster, service *api.Service, slots []orchestrator.Slot) {
	u.l.Lock()
	defer u.l.Unlock()

	id := service.Id

	if update, ok := u.updates[id]; ok {
		if service.Spec.EqualVT(update.newService.Spec) {
			// There's already an update working towards this goal.
			return
		}
		update.Cancel()
	}

	update := NewUpdater(u.store, u.restarts, cluster, service)
	u.updates[id] = update
	go func() {
		update.Run(ctx, slots)
		u.l.Lock()
		if u.updates[id] == update {
			delete(u.updates, id)
		}
		u.l.Unlock()
	}()
}

// CancelAll cancels all current updates.
func (u *Supervisor) CancelAll() {
	u.l.Lock()
	defer u.l.Unlock()

	for _, update := range u.updates {
		update.Cancel()
	}
}

// Updater updates a set of tasks to a new version.
type Updater struct {
	store      *store.MemoryStore
	watchQueue *watch.Queue
	restarts   *restart.Supervisor

	cluster    *api.Cluster
	newService *api.Service

	updatedTasks   map[string]time.Time // task ID to creation time
	updatedTasksMu sync.Mutex

	// stopChan signals to the state machine to stop running.
	stopChan chan struct{}
	// doneChan is closed when the state machine terminates.
	doneChan chan struct{}
}

// NewUpdater creates a new Updater.
func NewUpdater(store *store.MemoryStore, restartSupervisor *restart.Supervisor, cluster *api.Cluster, newService *api.Service) *Updater {
	return &Updater{
		store:        store,
		watchQueue:   store.WatchQueue(),
		restarts:     restartSupervisor,
		cluster:      cluster.Copy(),
		newService:   newService.Copy(),
		updatedTasks: make(map[string]time.Time),
		stopChan:     make(chan struct{}),
		doneChan:     make(chan struct{}),
	}
}

// Cancel cancels the current update immediately. It blocks until the cancellation is confirmed.
func (u *Updater) Cancel() {
	close(u.stopChan)
	<-u.doneChan
}

// Run starts the update and returns only once its complete or cancelled.
func (u *Updater) Run(ctx context.Context, slots []orchestrator.Slot) {
	defer close(u.doneChan)

	service := u.newService

	// If the update is in a PAUSED state, we should not do anything.
	if service.UpdateStatus != nil &&
		(service.UpdateStatus.State == api.UpdateStatus_PAUSED ||
			service.UpdateStatus.State == api.UpdateStatus_ROLLBACK_PAUSED) {
		return
	}

	var dirtySlots []orchestrator.Slot
	for _, slot := range slots {
		if u.isSlotDirty(slot) {
			dirtySlots = append(dirtySlots, slot)
		}
	}
	// Abort immediately if all tasks are clean.
	if len(dirtySlots) == 0 {
		if service.UpdateStatus != nil &&
			(service.UpdateStatus.State == api.UpdateStatus_UPDATING ||
				service.UpdateStatus.State == api.UpdateStatus_ROLLBACK_STARTED) {
			u.completeUpdate(ctx, service.Id)
		}
		return
	}

	// If there's no update in progress, we are starting one.
	if service.UpdateStatus == nil {
		u.startUpdate(ctx, service.Id)
	}

	var (
		monitoringPeriod time.Duration
		updateConfig     *api.UpdateConfig
	)

	if service.UpdateStatus != nil && service.UpdateStatus.State == api.UpdateStatus_ROLLBACK_STARTED {
		monitoringPeriod = defaults.Service.Rollback.Monitor.AsDuration()
		updateConfig = service.Spec.GetRollback()
		if updateConfig == nil {
			updateConfig = defaults.Service.Rollback
		}
	} else {
		monitoringPeriod = defaults.Service.Update.Monitor.AsDuration()
		updateConfig = service.Spec.GetUpdate()
		if updateConfig == nil {
			updateConfig = defaults.Service.Update
		}
	}

	parallelism := int(updateConfig.Parallelism)
	// AsDuration cannot fail, so validate explicitly to keep ignoring a
	// monitor period we cannot make sense of.
	if updateConfig.Monitor.IsValid() {
		monitoringPeriod = updateConfig.Monitor.AsDuration()
	}

	if parallelism == 0 {
		// TODO(aluzzardi): We could try to optimize unlimited parallelism by performing updates in a single
		// goroutine using a batch transaction.
		parallelism = len(dirtySlots)
	}

	// Start the workers.
	slotQueue := make(chan orchestrator.Slot)
	var wg sync.WaitGroup
	for range parallelism {
		wg.Go(func() {
			u.worker(ctx, slotQueue, updateConfig)
		})
	}

	var failedTaskWatch chan events.Event

	if updateConfig.FailureAction != api.UpdateConfig_CONTINUE {
		var cancelWatch func()
		failedTaskWatch, cancelWatch = state.Watch(
			u.store.WatchQueue(),
			api.EventUpdateTask{
				Task:   &api.Task{ServiceId: service.Id, Status: &api.TaskStatus{State: api.TaskState_RUNNING}},
				Checks: []api.TaskCheckFunc{api.TaskCheckServiceID, state.TaskCheckStateGreaterThan},
			},
		)
		defer cancelWatch()
	}

	stopped := false
	failedTasks := make(map[string]struct{})
	totalFailures := 0

	failureTriggersAction := func(failedTask *api.Task) bool {
		// Ignore tasks we have already seen as failures.
		if _, found := failedTasks[failedTask.Id]; found {
			return false
		}

		// If this failed/completed task is one that we
		// created as part of this update, we should
		// follow the failure action.
		u.updatedTasksMu.Lock()
		startedAt, found := u.updatedTasks[failedTask.Id]
		u.updatedTasksMu.Unlock()

		if found && (startedAt.IsZero() || time.Since(startedAt) <= monitoringPeriod) {
			failedTasks[failedTask.Id] = struct{}{}
			totalFailures++
			if float32(totalFailures)/float32(len(dirtySlots)) > updateConfig.MaxFailureRatio {
				switch updateConfig.FailureAction {
				case api.UpdateConfig_PAUSE:
					stopped = true
					message := fmt.Sprintf("update paused due to failure or early termination of task %s", failedTask.Id)
					u.pauseUpdate(ctx, service.Id, message)
					return true
				case api.UpdateConfig_ROLLBACK:
					// Never roll back a rollback
					if service.UpdateStatus != nil && service.UpdateStatus.State == api.UpdateStatus_ROLLBACK_STARTED {
						message := fmt.Sprintf("rollback paused due to failure or early termination of task %s", failedTask.Id)
						u.pauseUpdate(ctx, service.Id, message)
						return true
					}
					stopped = true
					message := fmt.Sprintf("update rolled back due to failure or early termination of task %s", failedTask.Id)
					u.rollbackUpdate(ctx, service.Id, message)
					return true
				}
			}
		}

		return false
	}

slotsLoop:
	for _, slot := range dirtySlots {
	retryLoop:
		for {
			// Wait for a worker to pick up the task or abort the update, whichever comes first.
			select {
			case <-u.stopChan:
				stopped = true
				break slotsLoop
			case ev := <-failedTaskWatch:
				if failureTriggersAction(ev.(api.EventUpdateTask).Task) {
					break slotsLoop
				}
			case slotQueue <- slot:
				break retryLoop
			}
		}
	}

	close(slotQueue)
	wg.Wait()

	if !stopped {
		// if a delay is set we need to monitor for a period longer than the delay
		// otherwise we will leave the monitorLoop before the task is done delaying
		if delay := updateConfig.Delay.AsDuration(); delay >= monitoringPeriod {
			monitoringPeriod = delay + 1*time.Second
		}
		// Keep watching for task failures for one more monitoringPeriod,
		// before declaring the update complete.
		doneMonitoring := time.After(monitoringPeriod)
	monitorLoop:
		for {
			select {
			case <-u.stopChan:
				stopped = true
				break monitorLoop
			case <-doneMonitoring:
				break monitorLoop
			case ev := <-failedTaskWatch:
				if failureTriggersAction(ev.(api.EventUpdateTask).Task) {
					break monitorLoop
				}
			}
		}
	}

	// TODO(aaronl): Potentially roll back the service if not enough tasks
	// have reached RUNNING by this point.

	if !stopped {
		u.completeUpdate(ctx, service.Id)
	}
}

func (u *Updater) worker(ctx context.Context, queue <-chan orchestrator.Slot, updateConfig *api.UpdateConfig) {
	for slot := range queue {
		// Do we have a task with the new spec in desired state = RUNNING?
		// If so, all we have to do to complete the update is remove the
		// other tasks. Or if we have a task with the new spec that has
		// desired state < RUNNING, advance it to running and remove the
		// other tasks.
		var (
			runningTask *api.Task
			cleanTask   *api.Task
		)
		for _, t := range slot {
			if !u.isTaskDirty(t) {
				if t.DesiredState == api.TaskState_RUNNING {
					runningTask = t
					break
				}
				if t.DesiredState < api.TaskState_RUNNING {
					cleanTask = t
				}
			}
		}
		if runningTask != nil {
			if err := u.useExistingTask(ctx, slot, runningTask); err != nil {
				log.G(ctx).WithError(err).Error("update failed")
			}
		} else if cleanTask != nil {
			if err := u.useExistingTask(ctx, slot, cleanTask); err != nil {
				log.G(ctx).WithError(err).Error("update failed")
			}
		} else {
			updated := orchestrator.NewTask(u.cluster, u.newService, slot[0].Slot, "")
			if orchestrator.IsGlobalService(u.newService) {
				updated = orchestrator.NewTask(u.cluster, u.newService, slot[0].Slot, slot[0].NodeId)
			}
			updated.DesiredState = api.TaskState_READY

			if err := u.updateTask(ctx, slot, updated, updateConfig.Order); err != nil {
				log.G(ctx).WithError(err).WithField("task.id", updated.Id).Error("update failed")
			}
		}

		if delay := updateConfig.Delay.AsDuration(); delay != 0 {
			select {
			case <-time.After(delay):
			case <-u.stopChan:
				return
			}
		}
	}
}

func (u *Updater) updateTask(ctx context.Context, slot orchestrator.Slot, updated *api.Task, order api.UpdateConfig_UpdateOrder) error {
	// Kick off the watch before even creating the updated task. This is in order to avoid missing any event.
	taskUpdates, cancel := state.Watch(u.watchQueue, api.EventUpdateTask{
		Task:   &api.Task{Id: updated.Id},
		Checks: []api.TaskCheckFunc{api.TaskCheckID},
	})
	defer cancel()

	// Create an empty entry for this task, so the updater knows a failure
	// should count towards the failure count. The timestamp is added
	// if/when the task reaches RUNNING.
	u.updatedTasksMu.Lock()
	u.updatedTasks[updated.Id] = time.Time{}
	u.updatedTasksMu.Unlock()

	startThenStop := false
	var delayStartCh <-chan struct{}
	// Atomically create the updated task and bring down the old one.
	err := u.store.Batch(func(batch *store.Batch) error {
		err := batch.Update(func(tx store.Tx) error {
			if store.GetService(tx, updated.ServiceId) == nil {
				return errors.New("service was deleted")
			}

			return store.CreateTask(tx, updated)
		})
		if err != nil {
			return err
		}

		if order == api.UpdateConfig_START_FIRST {
			delayStartCh = u.restarts.DelayStart(ctx, nil, nil, updated.Id, 0, false)
			startThenStop = true
		} else {
			oldTask, err := u.removeOldTasks(ctx, batch, slot)
			if err != nil {
				return err
			}
			delayStartCh = u.restarts.DelayStart(ctx, nil, oldTask, updated.Id, 0, true)
		}

		return nil

	})
	if err != nil {
		return err
	}

	if delayStartCh != nil {
		select {
		case <-delayStartCh:
		case <-u.stopChan:
			return nil
		}
	}

	// Wait for the new task to come up.
	// TODO(aluzzardi): Consider adding a timeout here.
	for {
		select {
		case e := <-taskUpdates:
			updated = e.(api.EventUpdateTask).Task
			if updated.Status.GetState() >= api.TaskState_RUNNING {
				u.updatedTasksMu.Lock()
				u.updatedTasks[updated.Id] = time.Now()
				u.updatedTasksMu.Unlock()

				if startThenStop && updated.Status.GetState() == api.TaskState_RUNNING {
					err := u.store.Batch(func(batch *store.Batch) error {
						_, err := u.removeOldTasks(ctx, batch, slot)
						if err != nil {
							log.G(ctx).WithError(err).WithField("task.id", updated.Id).Warning("failed to remove old task after starting replacement")
						}
						return nil
					})
					return err
				}
				return nil
			}
		case <-u.stopChan:
			return nil
		}
	}
}

func (u *Updater) useExistingTask(ctx context.Context, slot orchestrator.Slot, existing *api.Task) error {
	var removeTasks []*api.Task
	for _, t := range slot {
		if t != existing {
			removeTasks = append(removeTasks, t)
		}
	}
	if len(removeTasks) != 0 || existing.DesiredState != api.TaskState_RUNNING {
		var delayStartCh <-chan struct{}
		err := u.store.Batch(func(batch *store.Batch) error {
			var oldTask *api.Task
			if len(removeTasks) != 0 {
				var err error
				oldTask, err = u.removeOldTasks(ctx, batch, removeTasks)
				if err != nil {
					return err
				}
			}

			if existing.DesiredState != api.TaskState_RUNNING {
				delayStartCh = u.restarts.DelayStart(ctx, nil, oldTask, existing.Id, 0, true)
			}
			return nil
		})
		if err != nil {
			return err
		}

		if delayStartCh != nil {
			select {
			case <-delayStartCh:
			case <-u.stopChan:
				return nil
			}
		}
	}

	return nil
}

// removeOldTasks shuts down the given tasks and returns one of the tasks that
// was shut down, or an error.
func (u *Updater) removeOldTasks(_ context.Context, batch *store.Batch, removeTasks []*api.Task) (*api.Task, error) {
	var (
		lastErr     error
		removedTask *api.Task
	)
	for _, original := range removeTasks {
		if original.DesiredState > api.TaskState_RUNNING {
			continue
		}
		err := batch.Update(func(tx store.Tx) error {
			t := store.GetTask(tx, original.Id)
			if t == nil {
				return fmt.Errorf("task %s not found while trying to shut it down", original.Id)
			}
			if t.DesiredState > api.TaskState_RUNNING {
				return fmt.Errorf(
					"task %s was already shut down when reached by updater (state: %v)",
					original.Id, t.DesiredState,
				)
			}
			t.DesiredState = api.TaskState_SHUTDOWN
			return store.UpdateTask(tx, t)
		})
		if err != nil {
			lastErr = err
		} else {
			removedTask = original
		}
	}

	if removedTask == nil {
		return nil, lastErr
	}
	return removedTask, nil
}

func (u *Updater) isTaskDirty(t *api.Task) bool {
	var n *api.Node
	u.store.View(func(tx store.ReadTx) {
		n = store.GetNode(tx, t.NodeId)
	})
	return orchestrator.IsTaskDirty(u.newService, t, n)
}

func (u *Updater) isSlotDirty(slot orchestrator.Slot) bool {
	return len(slot) > 1 || (len(slot) == 1 && u.isTaskDirty(slot[0]))
}

func (u *Updater) startUpdate(ctx context.Context, serviceID string) {
	err := u.store.Update(func(tx store.Tx) error {
		service := store.GetService(tx, serviceID)
		if service == nil {
			return nil
		}
		if service.UpdateStatus != nil {
			return nil
		}

		service.UpdateStatus = &api.UpdateStatus{
			State:     api.UpdateStatus_UPDATING,
			Message:   "update in progress",
			StartedAt: ptypes.MustTimestampProto(time.Now()),
		}

		return store.UpdateService(tx, service)
	})

	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to mark update of service %s in progress", serviceID)
	}
}

func (u *Updater) pauseUpdate(ctx context.Context, serviceID, message string) {
	log.G(ctx).Debugf("pausing update of service %s", serviceID)

	err := u.store.Update(func(tx store.Tx) error {
		service := store.GetService(tx, serviceID)
		if service == nil {
			return nil
		}
		if service.UpdateStatus == nil {
			// The service was updated since we started this update
			return nil
		}

		if service.UpdateStatus.State == api.UpdateStatus_ROLLBACK_STARTED {
			service.UpdateStatus.State = api.UpdateStatus_ROLLBACK_PAUSED
		} else {
			service.UpdateStatus.State = api.UpdateStatus_PAUSED
		}
		service.UpdateStatus.Message = message

		return store.UpdateService(tx, service)
	})

	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to pause update of service %s", serviceID)
	}
}

func (u *Updater) rollbackUpdate(ctx context.Context, serviceID, message string) {
	log.G(ctx).Debugf("starting rollback of service %s", serviceID)

	err := u.store.Update(func(tx store.Tx) error {
		service := store.GetService(tx, serviceID)
		if service == nil {
			return nil
		}
		if service.UpdateStatus == nil {
			// The service was updated since we started this update
			return nil
		}

		service.UpdateStatus.State = api.UpdateStatus_ROLLBACK_STARTED
		service.UpdateStatus.Message = message

		if service.PreviousSpec == nil {
			return errors.New("cannot roll back service because no previous spec is available")
		}
		// Copy rather than assign: the spec and the previous spec would
		// otherwise be the same object, and PreviousSpec is cleared below.
		service.Spec = service.PreviousSpec.Copy()
		service.SpecVersion = service.PreviousSpecVersion.Copy()
		service.PreviousSpec = nil
		service.PreviousSpecVersion = nil

		return store.UpdateService(tx, service)
	})

	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to start rollback of service %s", serviceID)
		return
	}
}

func (u *Updater) completeUpdate(ctx context.Context, serviceID string) {
	log.G(ctx).Debugf("update of service %s complete", serviceID)

	err := u.store.Update(func(tx store.Tx) error {
		service := store.GetService(tx, serviceID)
		if service == nil {
			return nil
		}
		if service.UpdateStatus == nil {
			// The service was changed since we started this update
			return nil
		}
		if service.UpdateStatus.State == api.UpdateStatus_ROLLBACK_STARTED {
			service.UpdateStatus.State = api.UpdateStatus_ROLLBACK_COMPLETED
			service.UpdateStatus.Message = "rollback completed"
		} else {
			service.UpdateStatus.State = api.UpdateStatus_COMPLETED
			service.UpdateStatus.Message = "update completed"
		}
		service.UpdateStatus.CompletedAt = ptypes.MustTimestampProto(time.Now())

		return store.UpdateService(tx, service)
	})

	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to mark update of service %s complete", serviceID)
	}
}
