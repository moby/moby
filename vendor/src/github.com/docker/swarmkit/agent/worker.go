package agent

import (
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/boltdb/bolt"
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"golang.org/x/net/context"
)

// Worker implements the core task management logic and persistence. It
// coordinates the set of assignments with the executor.
type Worker interface {
	// Init prepares the worker for task assignment.
	Init(ctx context.Context) error

	// Assign assigns a complete set of tasks and secrets to a worker. Any task or secrets not included in
	// this set will be removed.
	Assign(ctx context.Context, assignments []*api.AssignmentChange) error

	// Updates updates an incremental set of tasks or secrets of the worker. Any task/secret not included
	// either in added or removed will remain untouched.
	Update(ctx context.Context, assignments []*api.AssignmentChange) error

	// Listen to updates about tasks controlled by the worker. When first
	// called, the reporter will receive all updates for all tasks controlled
	// by the worker.
	//
	// The listener will be removed if the context is cancelled.
	Listen(ctx context.Context, reporter StatusReporter)
}

// statusReporterKey protects removal map from panic.
type statusReporterKey struct {
	StatusReporter
}

type worker struct {
	db        *bolt.DB
	executor  exec.Executor
	listeners map[*statusReporterKey]struct{}
	secrets   *secrets

	taskManagers map[string]*taskManager
	mu           sync.RWMutex
}

func newWorker(db *bolt.DB, executor exec.Executor) *worker {
	return &worker{
		db:           db,
		executor:     executor,
		listeners:    make(map[*statusReporterKey]struct{}),
		taskManagers: make(map[string]*taskManager),
		secrets:      newSecrets(),
	}
}

// Init prepares the worker for assignments.
func (w *worker) Init(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	ctx = log.WithModule(ctx, "worker")

	// TODO(stevvooe): Start task cleanup process.

	// read the tasks from the database and start any task managers that may be needed.
	return w.db.Update(func(tx *bolt.Tx) error {
		return WalkTasks(tx, func(task *api.Task) error {
			if !TaskAssigned(tx, task.ID) {
				// NOTE(stevvooe): If tasks can survive worker restart, we need
				// to startup the controller and ensure they are removed. For
				// now, we can simply remove them from the database.
				if err := DeleteTask(tx, task.ID); err != nil {
					log.G(ctx).WithError(err).Errorf("error removing task %v", task.ID)
				}
				return nil
			}

			status, err := GetTaskStatus(tx, task.ID)
			if err != nil {
				log.G(ctx).WithError(err).Error("unable to read tasks status")
				return nil
			}

			task.Status = *status // merges the status into the task, ensuring we start at the right point.
			return w.startTask(ctx, tx, task)
		})
	})
}

// Assign assigns a full set of tasks and secrets to the worker.
// Any tasks not previously known will be started. Any tasks that are in the task set
// and already running will be updated, if possible. Any tasks currently running on
// the worker outside the task set will be terminated.
// Any secrets not in the set of assignments will be removed.
func (w *worker) Assign(ctx context.Context, assignments []*api.AssignmentChange) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log.G(ctx).WithFields(logrus.Fields{
		"len(assignments)": len(assignments),
	}).Debug("(*worker).Assign")

	// Need to update secrets before tasks, because tasks might depend on new secrets
	err := reconcileSecrets(ctx, w, assignments, true)
	if err != nil {
		return err
	}

	return reconcileTaskState(ctx, w, assignments, true)
}

// Update updates the set of tasks and secret for the worker.
// Tasks in the added set will be added to the worker, and tasks in the removed set
// will be removed from the worker
// Serets in the added set will be added to the worker, and secrets in the removed set
// will be removed from the worker.
func (w *worker) Update(ctx context.Context, assignments []*api.AssignmentChange) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log.G(ctx).WithFields(logrus.Fields{
		"len(assignments)": len(assignments),
	}).Debug("(*worker).Update")

	err := reconcileSecrets(ctx, w, assignments, false)
	if err != nil {
		return err
	}

	return reconcileTaskState(ctx, w, assignments, false)
}

func reconcileTaskState(ctx context.Context, w *worker, assignments []*api.AssignmentChange, fullSnapshot bool) error {
	var (
		updatedTasks []*api.Task
		removedTasks []*api.Task
	)
	for _, a := range assignments {
		if t := a.Assignment.GetTask(); t != nil {
			switch a.Action {
			case api.AssignmentChange_AssignmentActionUpdate:
				updatedTasks = append(updatedTasks, t)
			case api.AssignmentChange_AssignmentActionRemove:
				removedTasks = append(removedTasks, t)
			}
		}
	}

	log.G(ctx).WithFields(logrus.Fields{
		"len(updatedTasks)": len(updatedTasks),
		"len(removedTasks)": len(removedTasks),
	}).Debug("(*worker).reconcileTaskState")

	tx, err := w.db.Begin(true)
	if err != nil {
		log.G(ctx).WithError(err).Error("failed starting transaction against task database")
		return err
	}
	defer tx.Rollback()

	assigned := map[string]struct{}{}

	for _, task := range updatedTasks {
		log.G(ctx).WithFields(
			logrus.Fields{
				"task.id":           task.ID,
				"task.desiredstate": task.DesiredState}).Debug("assigned")
		if err := PutTask(tx, task); err != nil {
			return err
		}

		if err := SetTaskAssignment(tx, task.ID, true); err != nil {
			return err
		}

		if mgr, ok := w.taskManagers[task.ID]; ok {
			if err := mgr.Update(ctx, task); err != nil && err != ErrClosed {
				log.G(ctx).WithError(err).Error("failed updating assigned task")
			}
		} else {
			// we may have still seen the task, let's grab the status from
			// storage and replace it with our status, if we have it.
			status, err := GetTaskStatus(tx, task.ID)
			if err != nil {
				if err != errTaskUnknown {
					return err
				}

				// never seen before, register the provided status
				if err := PutTaskStatus(tx, task.ID, &task.Status); err != nil {
					return err
				}
			} else {
				task.Status = *status
			}
			w.startTask(ctx, tx, task)
		}

		assigned[task.ID] = struct{}{}
	}

	closeManager := func(tm *taskManager) {
		// when a task is no longer assigned, we shutdown the task manager for
		// it and leave cleanup to the sweeper.
		if err := tm.Close(); err != nil {
			log.G(ctx).WithError(err).Error("error closing task manager")
		}
	}

	removeTaskAssignment := func(taskID string) error {
		ctx := log.WithLogger(ctx, log.G(ctx).WithField("task.id", taskID))
		if err := SetTaskAssignment(tx, taskID, false); err != nil {
			log.G(ctx).WithError(err).Error("error setting task assignment in database")
		}
		return err
	}

	// If this was a complete set of assignments, we're going to remove all the remaining
	// tasks.
	if fullSnapshot {
		for id, tm := range w.taskManagers {
			if _, ok := assigned[id]; ok {
				continue
			}

			err := removeTaskAssignment(id)
			if err == nil {
				delete(w.taskManagers, id)
				go closeManager(tm)
			}
		}
	} else {
		// If this was an incremental set of assignments, we're going to remove only the tasks
		// in the removed set
		for _, task := range removedTasks {
			err := removeTaskAssignment(task.ID)
			if err != nil {
				continue
			}

			tm, ok := w.taskManagers[task.ID]
			if ok {
				delete(w.taskManagers, task.ID)
				go closeManager(tm)
			}
		}
	}

	return tx.Commit()
}

func reconcileSecrets(ctx context.Context, w *worker, assignments []*api.AssignmentChange, fullSnapshot bool) error {
	var (
		updatedSecrets []api.Secret
		removedSecrets []string
	)
	for _, a := range assignments {
		if s := a.Assignment.GetSecret(); s != nil {
			switch a.Action {
			case api.AssignmentChange_AssignmentActionUpdate:
				updatedSecrets = append(updatedSecrets, *s)
			case api.AssignmentChange_AssignmentActionRemove:
				removedSecrets = append(removedSecrets, s.ID)
			}

		}
	}

	log.G(ctx).WithFields(logrus.Fields{
		"len(updatedSecrets)": len(updatedSecrets),
		"len(removedSecrets)": len(removedSecrets),
	}).Debug("(*worker).reconcileSecrets")

	// If this was a complete set of secrets, we're going to clear the secrets map and add all of them
	if fullSnapshot {
		w.secrets.Reset()
	} else {
		w.secrets.Remove(removedSecrets)
	}
	w.secrets.Add(updatedSecrets...)

	return nil
}

func (w *worker) Listen(ctx context.Context, reporter StatusReporter) {
	w.mu.Lock()
	defer w.mu.Unlock()

	key := &statusReporterKey{reporter}
	w.listeners[key] = struct{}{}

	go func() {
		<-ctx.Done()
		w.mu.Lock()
		defer w.mu.Unlock()
		delete(w.listeners, key) // remove the listener if the context is closed.
	}()

	// report the current statuses to the new listener
	if err := w.db.View(func(tx *bolt.Tx) error {
		return WalkTaskStatus(tx, func(id string, status *api.TaskStatus) error {
			return reporter.UpdateTaskStatus(ctx, id, status)
		})
	}); err != nil {
		log.G(ctx).WithError(err).Errorf("failed reporting initial statuses to registered listener %v", reporter)
	}
}

func (w *worker) startTask(ctx context.Context, tx *bolt.Tx, task *api.Task) error {
	_, err := w.taskManager(ctx, tx, task) // side-effect taskManager creation.

	if err != nil {
		log.G(ctx).WithError(err).Error("failed to start taskManager")
	}

	// TODO(stevvooe): Add start method for taskmanager
	return nil
}

func (w *worker) taskManager(ctx context.Context, tx *bolt.Tx, task *api.Task) (*taskManager, error) {
	if tm, ok := w.taskManagers[task.ID]; ok {
		return tm, nil
	}

	tm, err := w.newTaskManager(ctx, tx, task)
	if err != nil {
		return nil, err
	}
	w.taskManagers[task.ID] = tm
	return tm, nil
}

func (w *worker) newTaskManager(ctx context.Context, tx *bolt.Tx, task *api.Task) (*taskManager, error) {
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("task.id", task.ID))

	ctlr, status, err := exec.Resolve(ctx, task, w.executor)
	if err := w.updateTaskStatus(ctx, tx, task.ID, status); err != nil {
		log.G(ctx).WithError(err).Error("error updating task status after controller resolution")
	}

	if err != nil {
		log.G(ctx).Error("controller resolution failed")
		return nil, err
	}

	return newTaskManager(ctx, task, ctlr, statusReporterFunc(func(ctx context.Context, taskID string, status *api.TaskStatus) error {
		w.mu.RLock()
		defer w.mu.RUnlock()

		return w.db.Update(func(tx *bolt.Tx) error {
			return w.updateTaskStatus(ctx, tx, taskID, status)
		})
	})), nil
}

// updateTaskStatus reports statuses to listeners, read lock must be held.
func (w *worker) updateTaskStatus(ctx context.Context, tx *bolt.Tx, taskID string, status *api.TaskStatus) error {
	if err := PutTaskStatus(tx, taskID, status); err != nil {
		log.G(ctx).WithError(err).Error("failed writing status to disk")
		return err
	}

	// broadcast the task status out.
	for key := range w.listeners {
		if err := key.StatusReporter.UpdateTaskStatus(ctx, taskID, status); err != nil {
			log.G(ctx).WithError(err).Errorf("failed updating status for reporter %v", key.StatusReporter)
		}
	}

	return nil
}
