package taskreaper

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/orchestrator"
	"github.com/moby/swarmkit/v2/manager/state"
	"github.com/moby/swarmkit/v2/manager/state/store"
)

const (
	// maxDirty is the size threshold for running a task pruning operation.
	maxDirty = 1000
	// reaperBatchingInterval is how often to prune old tasks.
	reaperBatchingInterval = 250 * time.Millisecond
)

// A TaskReaper deletes old tasks when more than TaskHistoryRetentionLimit tasks
// exist for the same service/instance or service/nodeid combination.
type TaskReaper struct {
	store *store.MemoryStore

	// closeOnce ensures that stopChan is closed only once
	closeOnce sync.Once

	// taskHistory is the number of tasks to keep
	taskHistory int64

	// List of slot tuples to be inspected for task history cleanup.
	dirty map[orchestrator.SlotTuple]struct{}

	// List of tasks collected for cleanup, which includes two kinds of tasks
	// - serviceless orphaned tasks
	// - tasks with desired state REMOVE that have already been shut down
	cleanup  []string
	stopChan chan struct{}
	doneChan chan struct{}

	// tickSignal is a channel that, if non-nil and available, will be written
	// to to signal that a tick has occurred. its sole purpose is for testing
	// code, to verify that take cleanup attempts are happening when they
	// should be.
	tickSignal chan struct{}
}

// New creates a new TaskReaper.
func New(store *store.MemoryStore) *TaskReaper {
	return &TaskReaper{
		store:    store,
		dirty:    make(map[orchestrator.SlotTuple]struct{}),
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
}

// Run is the TaskReaper's watch loop which collects candidates for cleanup.
// Task history is mainly used in task restarts but is also available for administrative purposes.
// Note that the task history is stored per-slot-per-service for replicated services
// and per-node-per-service for global services. History does not apply to serviceless tasks
// since they are not attached to a service. In addition, the TaskReaper watch loop is also
// responsible for cleaning up tasks associated with slots that were removed as part of
// service scale down or service removal.
func (tr *TaskReaper) Run(ctx context.Context) {
	watcher, watchCancel := state.Watch(tr.store.WatchQueue(), api.EventCreateTask{}, api.EventUpdateTask{}, api.EventUpdateCluster{})

	defer func() {
		close(tr.doneChan)
		watchCancel()
	}()

	var orphanedTasks []*api.Task
	var removeTasks []*api.Task
	tr.store.View(func(readTx store.ReadTx) {
		var err error

		clusters, err := store.FindClusters(readTx, store.ByName(store.DefaultClusterName))
		if err == nil && len(clusters) == 1 {
			tr.taskHistory = clusters[0].Spec.Orchestration.TaskHistoryRetentionLimit
		}

		// On startup, scan the entire store and inspect orphaned tasks from previous life.
		orphanedTasks, err = store.FindTasks(readTx, store.ByTaskState(api.TaskStateOrphaned))
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to find Orphaned tasks in task reaper init")
		}
		removeTasks, err = store.FindTasks(readTx, store.ByDesiredState(api.TaskStateRemove))
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to find tasks with desired state REMOVE in task reaper init")
		}
	})

	if len(orphanedTasks)+len(removeTasks) > 0 {
		for _, t := range orphanedTasks {
			// Do not reap service tasks immediately.
			// Let them go through the regular history cleanup process
			// of checking TaskHistoryRetentionLimit.
			if t.ServiceID != "" {
				continue
			}

			// Serviceless tasks can be cleaned up right away since they are not attached to a service.
			tr.cleanup = append(tr.cleanup, t.ID)
		}
		// tasks with desired state REMOVE that have progressed beyond COMPLETE or
		// haven't been assigned yet can be cleaned up right away
		for _, t := range removeTasks {
			if t.Status.State < api.TaskStateAssigned || t.Status.State >= api.TaskStateCompleted {
				tr.cleanup = append(tr.cleanup, t.ID)
			}
		}
		// Clean up tasks in 'cleanup' right away
		if len(tr.cleanup) > 0 {
			tr.tick()
		}
	}

	// Clean up when we hit TaskHistoryRetentionLimit or when the timer expires,
	// whichever happens first.
	//
	// Specifically, the way this should work:
	// - Create a timer and immediately stop it. We don't want to fire the
	//   cleanup routine yet, because we just did a cleanup as part of the
	//   initialization above.
	// - Launch into an event loop
	// - When we receive an event, handle the event as needed
	// - After receiving the event:
	//   - If minimum batch size (maxDirty) is exceeded with dirty + cleanup,
	//     then immediately launch into the cleanup routine
	//   - Otherwise, if the timer is stopped, start it (reset).
	// - If the timer expires and the timer channel is signaled, then Stop the
	//   timer (so that it will be ready to be started again as needed), and
	//   execute the cleanup routine (tick)
	timer := time.NewTimer(reaperBatchingInterval)
	timer.Stop()

	// If stop is somehow called AFTER the timer has expired, there will be a
	// value in the timer.C channel. If there is such a value, we should drain
	// it out. This select statement allows us to drain that value if it's
	// present, or continue straight through otherwise.
	select {
	case <-timer.C:
	default:
	}

	// keep track with a boolean of whether the timer is currently stopped
	isTimerStopped := true

	// Watch for:
	// 1. EventCreateTask for cleaning slots, which is the best time to cleanup that node/slot.
	// 2. EventUpdateTask for cleaning
	//    - serviceless orphaned tasks (when orchestrator updates the task status to ORPHANED)
	//    - tasks which have desired state REMOVE and have been shut down by the agent
	//      (these are tasks which are associated with slots removed as part of service
	//       remove or scale down)
	// 3. EventUpdateCluster for TaskHistoryRetentionLimit update.
	for {
		select {
		case event := <-watcher:
			switch v := event.(type) {
			case api.EventCreateTask:
				t := v.Task
				tr.dirty[orchestrator.SlotTuple{
					Slot:      t.Slot,
					ServiceID: t.ServiceID,
					NodeID:    t.NodeID,
				}] = struct{}{}
			case api.EventUpdateTask:
				t := v.Task
				// add serviceless orphaned tasks
				if t.Status.State >= api.TaskStateOrphaned && t.ServiceID == "" {
					tr.cleanup = append(tr.cleanup, t.ID)
				}
				// add tasks that are yet unassigned or have progressed beyond COMPLETE, with
				// desired state REMOVE. These tasks are associated with slots that were removed
				// as part of a service scale down or service removal.
				if t.DesiredState == api.TaskStateRemove && (t.Status.State < api.TaskStateAssigned || t.Status.State >= api.TaskStateCompleted) {
					tr.cleanup = append(tr.cleanup, t.ID)
				}
			case api.EventUpdateCluster:
				tr.taskHistory = v.Cluster.Spec.Orchestration.TaskHistoryRetentionLimit
			}

			if len(tr.dirty)+len(tr.cleanup) > maxDirty {
				// stop the timer, so we don't fire it. if we get another event
				// after we do this cleaning, we will reset the timer then
				timer.Stop()
				// if the timer had fired, drain out the value.
				select {
				case <-timer.C:
				default:
				}
				isTimerStopped = true
				tr.tick()
			} else if isTimerStopped {
				timer.Reset(reaperBatchingInterval)
				isTimerStopped = false
			}
		case <-timer.C:
			// we can safely ignore draining off of the timer channel, because
			// we already know that the timer has expired.
			isTimerStopped = true
			tr.tick()
		case <-tr.stopChan:
			// even though this doesn't really matter in this context, it's
			// good hygiene to drain the value.
			timer.Stop()
			select {
			case <-timer.C:
			default:
			}
			return
		}
	}
}

// taskInTerminalState returns true if task is in a terminal state.
func taskInTerminalState(task *api.Task) bool {
	return task.Status.State > api.TaskStateRunning
}

// taskWillNeverRun returns true if task will never reach running state.
func taskWillNeverRun(task *api.Task) bool {
	return task.Status.State < api.TaskStateAssigned && task.DesiredState > api.TaskStateRunning
}

// tick performs task history cleanup.
func (tr *TaskReaper) tick() {
	// this signals that a tick has occurred. it exists solely for testing.
	if tr.tickSignal != nil {
		// try writing to this channel, but if it's full, fall straight through
		// and ignore it.
		select {
		case tr.tickSignal <- struct{}{}:
		default:
		}
	}

	if len(tr.dirty) == 0 && len(tr.cleanup) == 0 {
		return
	}

	defer func() {
		tr.cleanup = nil
	}()

	deleteTasks := make(map[string]struct{})
	for _, tID := range tr.cleanup {
		deleteTasks[tID] = struct{}{}
	}

	// Check history of dirty tasks for cleanup.
	// Note: Clean out the dirty set at the end of this tick iteration
	// in all but one scenarios (documented below).
	// When tick() finishes, the tasks in the slot were either cleaned up,
	// or it was skipped because it didn't meet the criteria for cleaning.
	// Either way, we can discard the dirty set because future events on
	// that slot will cause the task to be readded to the dirty set
	// at that point.
	//
	// The only case when we keep the slot dirty is when there are more
	// than one running tasks present for a given slot.
	// In that case, we need to keep the slot dirty to allow it to be
	// cleaned when tick() is called next and one or more the tasks
	// in that slot have stopped running.
	tr.store.View(func(tx store.ReadTx) {
		for dirty := range tr.dirty {
			service := store.GetService(tx, dirty.ServiceID)
			if service == nil {
				delete(tr.dirty, dirty)
				continue
			}

			taskHistory := tr.taskHistory

			// If MaxAttempts is set, keep at least one more than
			// that number of tasks (this overrides TaskHistoryRetentionLimit).
			// This is necessary to reconstruct restart history when the orchestrator starts up.
			// TODO(aaronl): Consider hiding tasks beyond the normal
			// retention limit in the UI.
			// TODO(aaronl): There are some ways to cut down the
			// number of retained tasks at the cost of more
			// complexity:
			//   - Don't force retention of tasks with an older spec
			//     version.
			//   - Don't force retention of tasks outside of the
			//     time window configured for restart lookback.
			if service.Spec.Task.Restart != nil && service.Spec.Task.Restart.MaxAttempts > 0 {
				taskHistory = int64(service.Spec.Task.Restart.MaxAttempts) + 1
			}

			// Negative value for TaskHistoryRetentionLimit is an indication to never clean up task history.
			if taskHistory < 0 {
				delete(tr.dirty, dirty)
				continue
			}

			var historicTasks []*api.Task

			switch service.Spec.GetMode().(type) {
			case *api.ServiceSpec_Replicated:
				// Clean out the slot for which we received EventCreateTask.
				var err error
				historicTasks, err = store.FindTasks(tx, store.BySlot(dirty.ServiceID, dirty.Slot))
				if err != nil {
					continue
				}

			case *api.ServiceSpec_Global:
				// Clean out the node history in case of global services.
				tasksByNode, err := store.FindTasks(tx, store.ByNodeID(dirty.NodeID))
				if err != nil {
					continue
				}

				for _, t := range tasksByNode {
					if t.ServiceID == dirty.ServiceID {
						historicTasks = append(historicTasks, t)
					}
				}
			}

			if int64(len(historicTasks)) <= taskHistory {
				delete(tr.dirty, dirty)
				continue
			}

			// TODO(aaronl): This could filter for non-running tasks and use quickselect
			// instead of sorting the whole slice.
			// TODO(aaronl): This sort should really use lamport time instead of wall
			// clock time. We should store a Version in the Status field.
			sort.Sort(orchestrator.TasksByTimestamp(historicTasks))

			runningTasks := 0
			for _, t := range historicTasks {
				// Historical tasks can be considered for cleanup if:
				// 1. The task has reached a terminal state i.e. actual state beyond TaskStateRunning.
				// 2. The task has not yet become running and desired state is a terminal state i.e.
				// actual state not yet TaskStateAssigned and desired state beyond TaskStateRunning.
				if taskInTerminalState(t) || taskWillNeverRun(t) {
					deleteTasks[t.ID] = struct{}{}

					taskHistory++
					if int64(len(historicTasks)) <= taskHistory {
						break
					}
				} else {
					// all other tasks are counted as running.
					runningTasks++
				}
			}

			// The only case when we keep the slot dirty at the end of tick()
			// is when there are more than one running tasks present
			// for a given slot.
			// In that case, we keep the slot dirty to allow it to be
			// cleaned when tick() is called next and one or more of
			// the tasks in that slot have stopped running.
			if runningTasks <= 1 {
				delete(tr.dirty, dirty)
			}
		}
	})

	// Perform cleanup.
	if len(deleteTasks) > 0 {
		tr.store.Batch(func(batch *store.Batch) error {
			for taskID := range deleteTasks {
				batch.Update(func(tx store.Tx) error {
					return store.DeleteTask(tx, taskID)
				})
			}
			return nil
		})
	}
}

// Stop stops the TaskReaper and waits for the main loop to exit.
// Stop can be called in two cases. One when the manager is
// shutting down, and the other when the manager (the leader) is
// becoming a follower. Since these two instances could race with
// each other, we use closeOnce here to ensure that TaskReaper.Stop()
// is called only once to avoid a panic.
func (tr *TaskReaper) Stop() {
	tr.closeOnce.Do(func() {
		close(tr.stopChan)
	})
	<-tr.doneChan
}
