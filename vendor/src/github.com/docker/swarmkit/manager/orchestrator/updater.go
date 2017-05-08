package orchestrator

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/manager/state/watch"
	"github.com/docker/swarmkit/protobuf/ptypes"
)

// UpdateSupervisor supervises a set of updates. It's responsible for keeping track of updates,
// shutting them down and replacing them.
type UpdateSupervisor struct {
	store    *store.MemoryStore
	restarts *RestartSupervisor
	updates  map[string]*Updater
	l        sync.Mutex
}

// NewUpdateSupervisor creates a new UpdateSupervisor.
func NewUpdateSupervisor(store *store.MemoryStore, restartSupervisor *RestartSupervisor) *UpdateSupervisor {
	return &UpdateSupervisor{
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
func (u *UpdateSupervisor) Update(ctx context.Context, cluster *api.Cluster, service *api.Service, slots []slot) {
	u.l.Lock()
	defer u.l.Unlock()

	id := service.ID

	if update, ok := u.updates[id]; ok {
		if !update.isServiceDirty(service) {
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
func (u *UpdateSupervisor) CancelAll() {
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
	restarts   *RestartSupervisor

	cluster    *api.Cluster
	newService *api.Service

	// stopChan signals to the state machine to stop running.
	stopChan chan struct{}
	// doneChan is closed when the state machine terminates.
	doneChan chan struct{}
}

// NewUpdater creates a new Updater.
func NewUpdater(store *store.MemoryStore, restartSupervisor *RestartSupervisor, cluster *api.Cluster, newService *api.Service) *Updater {
	return &Updater{
		store:      store,
		watchQueue: store.WatchQueue(),
		restarts:   restartSupervisor,
		cluster:    cluster.Copy(),
		newService: newService.Copy(),
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
	}
}

// Cancel cancels the current update immediately. It blocks until the cancellation is confirmed.
func (u *Updater) Cancel() {
	close(u.stopChan)
	<-u.doneChan
}

// Run starts the update and returns only once its complete or cancelled.
func (u *Updater) Run(ctx context.Context, slots []slot) {
	defer close(u.doneChan)

	service := u.newService

	// If the update is in a PAUSED state, we should not do anything.
	if service.UpdateStatus != nil && service.UpdateStatus.State == api.UpdateStatus_PAUSED {
		return
	}

	var dirtySlots []slot
	for _, slot := range slots {
		if u.isSlotDirty(slot) {
			dirtySlots = append(dirtySlots, slot)
		}
	}
	// Abort immediately if all tasks are clean.
	if len(dirtySlots) == 0 {
		if service.UpdateStatus != nil && service.UpdateStatus.State == api.UpdateStatus_UPDATING {
			u.completeUpdate(ctx, service.ID)
		}
		return
	}

	// If there's no update in progress, we are starting one.
	if service.UpdateStatus == nil {
		u.startUpdate(ctx, service.ID)
	}

	parallelism := 0
	if service.Spec.Update != nil {
		parallelism = int(service.Spec.Update.Parallelism)
	}
	if parallelism == 0 {
		// TODO(aluzzardi): We could try to optimize unlimited parallelism by performing updates in a single
		// goroutine using a batch transaction.
		parallelism = len(dirtySlots)
	}

	// Start the workers.
	slotQueue := make(chan slot)
	wg := sync.WaitGroup{}
	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go func() {
			u.worker(ctx, slotQueue)
			wg.Done()
		}()
	}

	var failedTaskWatch chan events.Event

	if service.Spec.Update == nil || service.Spec.Update.FailureAction == api.UpdateConfig_PAUSE {
		var cancelWatch func()
		failedTaskWatch, cancelWatch = state.Watch(
			u.store.WatchQueue(),
			state.EventUpdateTask{
				Task:   &api.Task{ServiceID: service.ID, Status: api.TaskStatus{State: api.TaskStateRunning}},
				Checks: []state.TaskCheckFunc{state.TaskCheckServiceID, state.TaskCheckStateGreaterThan},
			},
		)
		defer cancelWatch()
	}

	stopped := false

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
				failedTask := ev.(state.EventUpdateTask).Task

				// If this failed/completed task has a spec matching
				// the one we're updating to, we should pause the
				// update.
				if !u.isTaskDirty(failedTask) {
					stopped = true
					message := fmt.Sprintf("update paused due to failure or early termination of task %s", failedTask.ID)
					u.pauseUpdate(ctx, service.ID, message)
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
		u.completeUpdate(ctx, service.ID)
	}
}

func (u *Updater) worker(ctx context.Context, queue <-chan slot) {
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
				if t.DesiredState == api.TaskStateRunning {
					runningTask = t
					break
				}
				if t.DesiredState < api.TaskStateRunning {
					cleanTask = t
				}
			}
		}
		if runningTask != nil {
			u.useExistingTask(ctx, slot, runningTask)
		} else if cleanTask != nil {
			u.useExistingTask(ctx, slot, cleanTask)
		} else {
			updated := newTask(u.cluster, u.newService, slot[0].Slot)
			updated.DesiredState = api.TaskStateReady
			if isGlobalService(u.newService) {
				updated.NodeID = slot[0].NodeID
			}

			if err := u.updateTask(ctx, slot, updated); err != nil {
				log.G(ctx).WithError(err).WithField("task.id", updated.ID).Error("update failed")
			}
		}

		if u.newService.Spec.Update != nil && (u.newService.Spec.Update.Delay.Seconds != 0 || u.newService.Spec.Update.Delay.Nanos != 0) {
			delay, err := ptypes.Duration(&u.newService.Spec.Update.Delay)
			if err != nil {
				log.G(ctx).WithError(err).Error("invalid update delay")
				continue
			}
			select {
			case <-time.After(delay):
			case <-u.stopChan:
				return
			}
		}
	}
}

func (u *Updater) updateTask(ctx context.Context, slot slot, updated *api.Task) error {
	// Kick off the watch before even creating the updated task. This is in order to avoid missing any event.
	taskUpdates, cancel := state.Watch(u.watchQueue, state.EventUpdateTask{
		Task:   &api.Task{ID: updated.ID},
		Checks: []state.TaskCheckFunc{state.TaskCheckID},
	})
	defer cancel()

	var delayStartCh <-chan struct{}
	// Atomically create the updated task and bring down the old one.
	_, err := u.store.Batch(func(batch *store.Batch) error {
		err := batch.Update(func(tx store.Tx) error {
			if err := store.CreateTask(tx, updated); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}

		u.removeOldTasks(ctx, batch, slot)

		for _, t := range slot {
			if t.DesiredState == api.TaskStateRunning {
				// Wait for the old task to stop or time out, and then set the new one
				// to RUNNING.
				delayStartCh = u.restarts.DelayStart(ctx, nil, t, updated.ID, 0, true)
				break
			}
		}

		return nil

	})
	if err != nil {
		return err
	}

	if delayStartCh != nil {
		<-delayStartCh
	}

	// Wait for the new task to come up.
	// TODO(aluzzardi): Consider adding a timeout here.
	for {
		select {
		case e := <-taskUpdates:
			updated = e.(state.EventUpdateTask).Task
			if updated.Status.State >= api.TaskStateRunning {
				return nil
			}
		case <-u.stopChan:
			return nil
		}
	}
}

func (u *Updater) useExistingTask(ctx context.Context, slot slot, existing *api.Task) {
	var removeTasks []*api.Task
	for _, t := range slot {
		if t != existing {
			removeTasks = append(removeTasks, t)
		}
	}
	if len(removeTasks) != 0 || existing.DesiredState != api.TaskStateRunning {
		_, err := u.store.Batch(func(batch *store.Batch) error {
			u.removeOldTasks(ctx, batch, removeTasks)

			if existing.DesiredState != api.TaskStateRunning {
				err := batch.Update(func(tx store.Tx) error {
					t := store.GetTask(tx, existing.ID)
					if t == nil {
						return fmt.Errorf("task %s not found while trying to start it", existing.ID)
					}
					if t.DesiredState >= api.TaskStateRunning {
						return fmt.Errorf("task %s was already started when reached by updater", existing.ID)
					}
					t.DesiredState = api.TaskStateRunning
					return store.UpdateTask(tx, t)
				})
				if err != nil {
					log.G(ctx).WithError(err).Errorf("starting task %s failed", existing.ID)
				}
			}
			return nil
		})
		if err != nil {
			log.G(ctx).WithError(err).Error("updater batch transaction failed")
		}
	}
}

func (u *Updater) removeOldTasks(ctx context.Context, batch *store.Batch, removeTasks []*api.Task) {
	for _, original := range removeTasks {
		err := batch.Update(func(tx store.Tx) error {
			t := store.GetTask(tx, original.ID)
			if t == nil {
				return fmt.Errorf("task %s not found while trying to shut it down", original.ID)
			}
			if t.DesiredState > api.TaskStateRunning {
				return fmt.Errorf("task %s was already shut down when reached by updater", original.ID)
			}
			t.DesiredState = api.TaskStateShutdown
			return store.UpdateTask(tx, t)
		})
		if err != nil {
			log.G(ctx).WithError(err).Errorf("shutting down stale task %s failed", original.ID)
		}
	}
}

func (u *Updater) isTaskDirty(t *api.Task) bool {
	return !reflect.DeepEqual(u.newService.Spec.Task, t.Spec) ||
		(t.Endpoint != nil && !reflect.DeepEqual(u.newService.Spec.Endpoint, t.Endpoint.Spec))
}

func (u *Updater) isServiceDirty(service *api.Service) bool {
	return !reflect.DeepEqual(u.newService.Spec.Task, service.Spec.Task) ||
		!reflect.DeepEqual(u.newService.Spec.Endpoint, service.Spec.Endpoint)
}

func (u *Updater) isSlotDirty(slot slot) bool {
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

		service.UpdateStatus.State = api.UpdateStatus_PAUSED
		service.UpdateStatus.Message = message

		return store.UpdateService(tx, service)
	})

	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to pause update of service %s", serviceID)
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

		service.UpdateStatus.State = api.UpdateStatus_COMPLETED
		service.UpdateStatus.Message = "update completed"
		service.UpdateStatus.CompletedAt = ptypes.MustTimestampProto(time.Now())

		return store.UpdateService(tx, service)
	})

	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to mark update of service %s complete", serviceID)
	}
}
