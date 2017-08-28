package taskreaper

import (
	"sort"
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/orchestrator"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
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
	// taskHistory is the number of tasks to keep
	taskHistory int64
	dirty       map[orchestrator.SlotTuple]struct{}
	orphaned    []string
	stopChan    chan struct{}
	doneChan    chan struct{}
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

// Run is the TaskReaper's main loop.
func (tr *TaskReaper) Run(ctx context.Context) {
	watcher, watchCancel := state.Watch(tr.store.WatchQueue(), api.EventCreateTask{}, api.EventUpdateTask{}, api.EventUpdateCluster{})

	defer func() {
		close(tr.doneChan)
		watchCancel()
	}()

	var tasks []*api.Task
	tr.store.View(func(readTx store.ReadTx) {
		var err error

		clusters, err := store.FindClusters(readTx, store.ByName(store.DefaultClusterName))
		if err == nil && len(clusters) == 1 {
			tr.taskHistory = clusters[0].Spec.Orchestration.TaskHistoryRetentionLimit
		}

		tasks, err = store.FindTasks(readTx, store.ByTaskState(api.TaskStateOrphaned))
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to find Orphaned tasks in task reaper init")
		}
	})

	if len(tasks) > 0 {
		for _, t := range tasks {
			// Do not reap service tasks immediately
			if t.ServiceID != "" {
				continue
			}

			tr.orphaned = append(tr.orphaned, t.ID)
		}

		if len(tr.orphaned) > 0 {
			tr.tick()
		}
	}

	timer := time.NewTimer(reaperBatchingInterval)

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
				if t.Status.State >= api.TaskStateOrphaned && t.ServiceID == "" {
					tr.orphaned = append(tr.orphaned, t.ID)
				}
			case api.EventUpdateCluster:
				tr.taskHistory = v.Cluster.Spec.Orchestration.TaskHistoryRetentionLimit
			}

			if len(tr.dirty)+len(tr.orphaned) > maxDirty {
				timer.Stop()
				tr.tick()
			} else {
				timer.Reset(reaperBatchingInterval)
			}
		case <-timer.C:
			timer.Stop()
			tr.tick()
		case <-tr.stopChan:
			timer.Stop()
			return
		}
	}
}

func (tr *TaskReaper) tick() {
	if len(tr.dirty) == 0 && len(tr.orphaned) == 0 {
		return
	}

	defer func() {
		tr.orphaned = nil
	}()

	deleteTasks := make(map[string]struct{})
	for _, tID := range tr.orphaned {
		deleteTasks[tID] = struct{}{}
	}
	tr.store.View(func(tx store.ReadTx) {
		for dirty := range tr.dirty {
			service := store.GetService(tx, dirty.ServiceID)
			if service == nil {
				continue
			}

			taskHistory := tr.taskHistory

			// If MaxAttempts is set, keep at least one more than
			// that number of tasks. This is necessary reconstruct
			// restart history when the orchestrator starts up.
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

			if taskHistory < 0 {
				continue
			}

			var historicTasks []*api.Task

			switch service.Spec.GetMode().(type) {
			case *api.ServiceSpec_Replicated:
				var err error
				historicTasks, err = store.FindTasks(tx, store.BySlot(dirty.ServiceID, dirty.Slot))
				if err != nil {
					continue
				}

			case *api.ServiceSpec_Global:
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
				continue
			}

			// TODO(aaronl): This could filter for non-running tasks and use quickselect
			// instead of sorting the whole slice.
			// TODO(aaronl): This sort should really use lamport time instead of wall
			// clock time. We should store a Version in the Status field.
			sort.Sort(orchestrator.TasksByTimestamp(historicTasks))

			runningTasks := 0
			for _, t := range historicTasks {
				if t.DesiredState <= api.TaskStateRunning || t.Status.State <= api.TaskStateRunning {
					// Don't delete running tasks
					runningTasks++
					continue
				}

				deleteTasks[t.ID] = struct{}{}

				taskHistory++
				if int64(len(historicTasks)) <= taskHistory {
					break
				}
			}

			if runningTasks <= 1 {
				delete(tr.dirty, dirty)
			}
		}
	})

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
func (tr *TaskReaper) Stop() {
	close(tr.stopChan)
	<-tr.doneChan
}
